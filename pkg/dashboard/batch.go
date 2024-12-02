/*
 Copyright 2024 Juicedata Inc

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

package dashboard

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
	"github.com/juicedata/juicefs-csi-driver/pkg/util/resource"
)

var batchLog = klog.NewKlogr().WithName("batch")

func (api *API) getNodes() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeList corev1.NodeList
		err := api.cachedReader.List(c, &nodeList, &client.ListOptions{})
		if err != nil {
			c.String(500, "list nodes error %v", err)
			return
		}
		c.IndentedJSON(200, nodeList.Items)
	}
}

func (api *API) upgradePods() gin.HandlerFunc {
	return func(c *gin.Context) {
		body := &struct {
			NodeName    string   `json:"nodeName"`
			ReCreate    bool     `json:"recreate,omitempty"`
			Worker      int      `json:"worker,omitempty"`
			IgnoreError bool     `json:"ignoreError,omitempty"`
			UniqueIds   []string `json:"uniqueIds,omitempty"`
		}{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		nodeName := body.NodeName
		recreate := body.ReCreate
		var needCreate bool
		jobName := common.GenUpgradeJobName()

		job, err := api.client.BatchV1().Jobs(getSysNamespace()).Get(c, jobName, metav1.GetOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			c.String(500, "get job error %v", err)
			return
		}
		if k8serrors.IsNotFound(err) {
			needCreate = true
		}

		// if job already completed (fail or succeed), create it again
		if job.Status.Succeeded == 1 || job.Status.Failed != 0 {
			needCreate = true
			if err = api.client.BatchV1().Jobs(job.Namespace).Delete(c, job.Name, metav1.DeleteOptions{
				PropagationPolicy: util.ToPtr(metav1.DeletePropagationBackground),
			}); err != nil {
				c.String(500, "delete job error %v", err)
				return
			}

			// wait for job deleted
			t := time.NewTicker(1 * time.Second)
			for range t.C {
				_, err = api.client.BatchV1().Jobs(getSysNamespace()).Get(c, jobName, metav1.GetOptions{})
				if err != nil && k8serrors.IsNotFound(err) {
					t.Stop()
					break
				}
			}
		}
		if needCreate {
			var csis corev1.PodList
			s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "juicefs-csi-driver",
					"app":                    "juicefs-csi-node",
				},
			})
			err = api.cachedReader.List(c, &csis, &client.ListOptions{LabelSelector: s})
			if err != nil {
				c.String(500, "list pods error %v", err)
				return
			}
			pods, err := api.getUpgradePods(c, body.UniqueIds, body.NodeName, body.ReCreate)
			if err != nil {
				c.String(500, "get upgrade pods error %v", err)
				return
			}
			batchConfig := config.NewBatchConfig(pods, body.Worker, body.IgnoreError, body.ReCreate, body.NodeName, body.UniqueIds, csis.Items)
			cfg, err := config.SaveUpgradeConfig(c, api.k8sclient, config.UpgradeConfigMapName, batchConfig)
			if err != nil {
				c.String(500, "save upgrade config error %v", err)
				return
			}

			newJob := newUpgradeJob(nodeName, recreate, body.UniqueIds)
			job, err = api.client.BatchV1().Jobs(newJob.Namespace).Create(c, newJob, metav1.CreateOptions{})
			if err != nil {
				batchLog.Error(err, "create job error")
				c.String(500, "create job error %v", err)
				return
			}
			if cfg, err = api.client.CoreV1().ConfigMaps(cfg.Namespace).Get(c, cfg.Name, metav1.GetOptions{}); err != nil {
				c.String(500, "get configmap error %v", err)
				return
			}
			SetJobAsConfigMapOwner(cfg, job)
			if _, err := api.client.CoreV1().ConfigMaps(cfg.Namespace).Update(c, cfg, metav1.UpdateOptions{}); err != nil {
				batchLog.Error(err, "update configmap error")
				c.String(500, "update configmap error %v", err)
				return
			}
		}
		c.IndentedJSON(200, map[string]string{
			"jobName": jobName,
		})
	}
}

func (api *API) getUpgradeStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobName := common.GenUpgradeJobName()
		job, err := api.client.BatchV1().Jobs(getSysNamespace()).Get(c, jobName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				c.IndentedJSON(200, batchv1.Job{})
				return
			}
			c.String(500, "get job error %v", err)
			return
		}
		c.IndentedJSON(200, job)
	}
}

func (api *API) clearUpgradeStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobName := common.GenUpgradeJobName()
		err := api.client.BatchV1().Jobs(getSysNamespace()).Delete(c, jobName, metav1.DeleteOptions{
			PropagationPolicy: util.ToPtr(metav1.DeletePropagationBackground),
		})
		if err != nil && !k8serrors.IsNotFound(err) {
			c.String(500, "delete job error %v", err)
			return
		}
		return
	}
}

func (api *API) getUpgradeJobLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobName := c.Query("job")
		job, err := api.client.BatchV1().Jobs(getSysNamespace()).Get(c, jobName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				c.String(400, "not found")
				return
			}
			c.String(500, "get job error %v", err)
			return
		}
		s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.JfsUpgradeJobLabelKey: common.JfsUpgradeJobLabelValue,
				"job-name":                   jobName,
			},
		})
		podList, err := api.client.CoreV1().Pods(job.Namespace).List(c, metav1.ListOptions{LabelSelector: s.String()})
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		if len(podList.Items) == 0 {
			c.String(404, "pod of job not found")
			return
		}
		pod := podList.Items[0]
		download := c.Query("download")
		if download == "true" {
			c.Header("Content-Disposition", "attachment; filename="+job.Name+".log")
		}
		logs, err := api.client.CoreV1().Pods(job.Namespace).GetLogs(jobName, &corev1.PodLogOptions{
			Container: pod.Spec.Containers[0].Name,
		}).DoRaw(c)
		if err != nil {
			c.String(500, "get pod logs error %v", err)
			return
		}
		c.String(200, string(logs))
	}
}

func (api *API) watchUpgradeJobLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		websocket.Handler(func(ws *websocket.Conn) {
			jobName := c.Query("job")
			var (
				job     *batchv1.Job
				jobPod  *corev1.Pod
				podList *corev1.PodList
				err     error
				t       = time.NewTicker(2 * time.Second)
			)
			ctx, cancel := context.WithTimeout(c, 2*time.Minute)
			defer cancel()
			for {
				job, err = api.client.BatchV1().Jobs(getSysNamespace()).Get(c, jobName, metav1.GetOptions{})
				if err == nil && job.DeletionTimestamp == nil {
					s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.JfsUpgradeJobLabelKey: common.JfsUpgradeJobLabelValue,
							common.JfsUpgradePodLabelKey: jobName,
						},
					})
					podList, err = api.client.CoreV1().Pods(job.Namespace).List(c, metav1.ListOptions{LabelSelector: s.String()})
					if err == nil && len(podList.Items) != 0 {
						batchLog.V(1).Info("get pod status", "pod", podList.Items[0].Name, "status", podList.Items[0].Status.Phase)
						if podList.Items[0].Status.Phase != corev1.PodPending {
							jobPod = &podList.Items[0]
							t.Stop()
							break
						}
					}
				}
				select {
				case <-ctx.Done():
					c.String(500, "get job or list pod timeout")
					batchLog.Info("get job or list pod timeout", "job", jobName)
					_, _ = ws.Write([]byte(fmt.Sprintf("Upgrade timeout, job for upgrade is not ready, please check job [%s] in [%s] and try again later.", jobName, getSysNamespace())))
					t.Stop()
					return
				case <-t.C:
					break
				}
			}
			defer ws.Close()
			req := api.client.CoreV1().Pods(jobPod.Namespace).GetLogs(jobPod.Name, &corev1.PodLogOptions{
				Container: jobPod.Spec.Containers[0].Name,
				Follow:    true,
			})
			stream, err := req.Stream(c.Request.Context())
			if err != nil {
				return
			}
			wr := newLogPipe(c.Request.Context(), ws, stream)
			_, err = io.Copy(wr, wr)
			if err != nil {
				return
			}
		}).ServeHTTP(c.Writer, c.Request)
	}
}

func newUpgradeJob(nodeName string, recreate bool, uniqueIds []string) *batchv1.Job {
	sysNamespace := getSysNamespace()
	cmds := []string{"juicefs-csi-dashboard", "upgrade"}
	ttl := int32(300)
	sa := "juicefs-csi-dashboard-sa"
	if os.Getenv("JUICEFS_CSI_DASHBOARD_SA") != "" {
		sa = os.Getenv("JUICEFS_CSI_DASHBOARD_SA")
	}
	recreateLabel := "false"
	if recreate {
		recreateLabel = "true"
	}
	jobName := common.GenUpgradeJobName()
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: sysNamespace,
			Labels: map[string]string{
				common.JfsUpgradeJobLabelKey: common.JfsUpgradeJobLabelValue,
			},
			Annotations: map[string]string{
				common.JfsUpgradeNodeName:     nodeName,
				common.JfsUpgradeRecreateName: recreateLabel,
				common.JfsUpgradeUniqueIds:    strings.Join(uniqueIds, "/"),
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:  util.ToPtr(int32(1)),
			Completions:  util.ToPtr(int32(1)),
			BackoffLimit: util.ToPtr(int32(1)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.JfsUpgradeJobLabelKey: common.JfsUpgradeJobLabelValue,
						common.JfsUpgradePodLabelKey: jobName,
					},
					Annotations: map[string]string{
						common.JfsUpgradeNodeName:     nodeName,
						common.JfsUpgradeRecreateName: recreateLabel,
						common.JfsUpgradeUniqueIds:    strings.Join(uniqueIds, "/"),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "juicefs-upgrade",
						Image:   os.Getenv("DASHBOARD_IMAGE"),
						Command: cmds,
						Env:     []corev1.EnvVar{{Name: "SYS_NAMESPACE", Value: sysNamespace}},
					}},
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: sa,
				},
			},
			TTLSecondsAfterFinished: &ttl,
		},
	}
}

func (api *API) getUpgradePods(ctx context.Context, uniqueIds []string, nodeName string, recreate bool) ([]corev1.Pod, error) {
	var pods corev1.PodList
	ls := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": "juicefs-mount",
		},
	}
	if len(uniqueIds) != 0 {
		ls.MatchExpressions = []metav1.LabelSelectorRequirement{{
			Key:      common.PodUniqueIdLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   uniqueIds,
		}}
	}
	s, _ := metav1.LabelSelectorAsSelector(ls)
	listOptions := client.ListOptions{
		LabelSelector: s,
	}
	if nodeName != "" {
		fieldSelector := fields.Set{"spec.nodeName": nodeName}.AsSelector()
		listOptions.FieldSelector = fieldSelector
	}
	err := api.cachedReader.List(ctx, &pods, &listOptions)
	if err != nil {
		return nil, err
	}

	podsToUpgrade := resource.FilterPodsToUpgrade(ctx, api.k8sclient, pods, recreate)

	// load config
	if err := config.LoadFromConfigMap(ctx, api.k8sclient); err != nil {
		return nil, err
	}

	var needUpdatePods []corev1.Pod
	for _, pod := range podsToUpgrade {
		po := pod
		diff, err := DiffConfig(ctx, api.k8sclient, &po)
		if err != nil {
			return nil, err
		}
		if diff {
			needUpdatePods = append(needUpdatePods, po)
		}
	}
	return needUpdatePods, nil
}
