/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/go-logr/logr"
	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sampleoperatorv1alpha1 "github.com/khiraiwa/sample-fargate-operator/api/v1alpha1"
)

// FargateTaskReconciler reconciles a FargateTask object
type FargateTaskReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

const myFinalizerName = "sample-fargate-operator.finalizer"

// +kubebuilder:rbac:groups=sample-operator.hiraken.cf,resources=fargatetasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sample-operator.hiraken.cf,resources=fargatetasks/status,verbs=get;update;patch

func (r *FargateTaskReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("fargatetask", req.NamespacedName)

	// your logic here
	log.V(1).Info("=== Reconsiling FargateTask")

	instance := &sampleoperatorv1alpha1.FargateTask{}
	if err := r.Get(context.TODO(), req.NamespacedName, instance); err != nil {
		// 既に instance が削除済みの場合はスキップ。
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		log.Error(err, "error occured.")
		return ctrl.Result{}, err
	}

	region := instance.Spec.Region
	cluster := instance.Spec.Cluster
	count := instance.Spec.Count
	subnets := instance.Spec.NetworkConfiguration.AwsVpcConfiguration.Subnets
	securityGroups := instance.Spec.NetworkConfiguration.AwsVpcConfiguration.SecurityGroups
	assignedPublicIP := instance.Spec.NetworkConfiguration.AwsVpcConfiguration.AssignPublicIp
	taskDefinition := instance.Spec.TaskDefinition

	// エラーチェック (手抜き...)
	if region == "" || cluster == "" || count < 1 || len(subnets) < 2 || len(securityGroups) < 1 || assignedPublicIP == "" || taskDefinition == "" {
		err := fmt.Errorf("err: %s", "some fields are invalid.")
		return ctrl.Result{}, err
	}

	sess := session.Must(session.NewSession())
	svc := ecs.New(sess, aws.NewConfig().WithRegion(region))

	if instance.ObjectMeta.DeletionTimestamp.IsZero() { // instance のステータスが Deleting の状態でない場合

		// 既に Fargate のタスクが起動済みである場合はスキップ
		if instance.Status.Phase == sampleoperatorv1alpha1.PhaseDone {
			return ctrl.Result{}, nil
		}

		// FargateTask オブジェクト作成時の処理
		log.V(1).Info("==== Runnging FargateTask")
		if err := r.runFargateTasks(instance, svc, cluster, count, assignedPublicIP, subnets, securityGroups, taskDefinition); err != nil {
			return ctrl.Result{}, err
		}
	} else { // instance のステータスが Deleting の状態である場合

		// FargateTask オブジェクト削除時の処理
		log.V(1).Info("==== Stopping FargateTask")
		if err := r.stopFargateTasks(instance, svc, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *FargateTaskReconciler) runFargateTasks(instance *sampleoperatorv1alpha1.FargateTask, svc *ecs.ECS,
	cluster string, count int64, assignedPublicIP string, subnets []string,
	securityGroups []string, taskDefinition string) error {

	input := &ecs.RunTaskInput{
		Cluster:    aws.String(cluster),
		Count:      aws.Int64(count),
		LaunchType: aws.String("FARGATE"),
		NetworkConfiguration: &ecs.NetworkConfiguration{
			AwsvpcConfiguration: &ecs.AwsVpcConfiguration{
				AssignPublicIp: aws.String(assignedPublicIP),
				Subnets:        aws.StringSlice(subnets),
				SecurityGroups: aws.StringSlice(securityGroups),
			},
		},
		TaskDefinition: aws.String(taskDefinition),
	}
	result, err := svc.RunTask(input)

	if err != nil {
		log.Error(err, "RunTask() failed.")
		return err
	}

	var taskArns []string
	for _, task := range result.Tasks {
		taskArns = append(taskArns, *task.TaskArn)
	}

	// instance 削除の際に終了処理で Fargate のタスクを削除できるように status.taskArns にタスクの Arn を追加する。
	instance.Status.TaskArns = taskArns

	// 既に Fargate のタスクが起動済みであるため、status.phase に PhaseDone を設定する。
	instance.Status.Phase = sampleoperatorv1alpha1.PhaseDone

	// metadata.finalizer を追加する。metadata.finalizer が存在する限り、instance の削除を行った場合に Deleting のままとなる。
	instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, myFinalizerName)

	if err = r.Update(context.TODO(), instance); err != nil {
		log.Error(err, "updating instance failed.")
		return err
	}
	return nil
}

func (r *FargateTaskReconciler) stopFargateTasks(instance *sampleoperatorv1alpha1.FargateTask,
	svc *ecs.ECS, cluster string) error {

	for _, taskArn := range instance.Status.TaskArns {
		input := &ecs.StopTaskInput{
			Cluster: aws.String(cluster),
			Task:    aws.String(taskArn),
		}
		_, err := svc.StopTask(input)
		if err != nil {
			log.Error(err, "StopTask() failed.")
			return err
		}
	}
	// instance.finalizer を削除する。削除後、instance は削除される。
	instance.ObjectMeta.Finalizers = r.removeString(instance.ObjectMeta.Finalizers, myFinalizerName)
	if err := r.Update(context.TODO(), instance); err != nil {
		log.Error(err, "updating instance failed.")
		return err
	}
	return nil
}

func (r *FargateTaskReconciler) removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func (r *FargateTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sampleoperatorv1alpha1.FargateTask{}).
		Complete(r)
}
