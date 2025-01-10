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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/websocket"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/juicedata/juicefs-csi-driver/pkg/common"
	"github.com/juicedata/juicefs-csi-driver/pkg/config"
	"github.com/juicedata/juicefs-csi-driver/pkg/dashboard/utils"
	"github.com/juicedata/juicefs-csi-driver/pkg/util"
)

var batchLog = klog.NewKlogr().WithName("batch")

type ListJobResult struct {
	Total    int           `json:"total"`
	Continue string        `json:"continue"`
	Jobs     []*UpgradeJob `json:"jobs"`
}

type UpgradeJob struct {
	Job    *batchv1.Job        `json:"job"`
	Config *config.BatchConfig `json:"config"`
}

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

func (api *API) createUpgradeJob() gin.HandlerFunc {
	return func(c *gin.Context) {
		createJobBody := struct {
			JobName     string `json:"jobName,omitempty"`
			NodeName    string `json:"nodeName,omitempty"`
			Recreate    bool   `json:"recreate,omitempty"`
			Worker      int    `json:"worker,omitempty"`
			IgnoreError bool   `json:"ignoreError,omitempty"`
			UniqueId    string `json:"uniqueId,omitempty"`
		}{}
		if err := c.ShouldBindJSON(&createJobBody); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		jobName := createJobBody.JobName
		if jobName == "" {
			jobName = GenUpgradeJobName()
		}

		cmName := GenUpgradeConfig(jobName)
		pods, err := api.podSvc.ListUpgradePods(c, createJobBody.UniqueId, createJobBody.NodeName, createJobBody.Recreate)
		if err != nil {
			c.String(500, "get upgrade pods error %v", err)
			return
		}
		csiNodes, err := api.podSvc.ListCSINodePod(c, createJobBody.NodeName)
		if err != nil {
			c.String(500, "get csi node pods error %v", err)
			return
		}

		batchConfig := config.NewBatchConfig(pods, createJobBody.Worker, createJobBody.IgnoreError, createJobBody.Recreate, createJobBody.NodeName, createJobBody.UniqueId, csiNodes)
		cfg, err := config.CreateUpgradeConfig(c, api.client, cmName, batchConfig)
		if err != nil {
			c.String(500, "save upgrade config error %v", err)
			return
		}

		newJob := newUpgradeJob(jobName)
		job, err := api.client.BatchV1().Jobs(newJob.Namespace).Create(c, newJob, metav1.CreateOptions{})
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
		c.IndentedJSON(200, map[string]string{
			"jobName": jobName,
		})
	}
}

func (api *API) listUpgradeJobs() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobs, err := api.jobSvc.ListAllBatchJobs(c)
		if err != nil {
			c.String(500, "list jobs error %v", err)
			return
		}

		configs, err := api.getAllUpgradeConfig(c)
		if err != nil {
			c.String(500, "get all upgrade config error %v", err)
			return
		}

		result := &ListJobResult{
			jobs.Total,
			jobs.Continue,
			make([]*UpgradeJob, 0),
		}

		for i := range jobs.Jobs {
			result.Jobs = append(result.Jobs, &UpgradeJob{
				Job:    &jobs.Jobs[i],
				Config: configs[jobs.Jobs[i].Labels[common.JfsUpgradeConfig]],
			})
		}
		c.IndentedJSON(200, result)
	}
}

func (api *API) getUpgradeJob() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobName := c.Param("jobName")
		job, err := api.client.BatchV1().Jobs(config.Namespace).Get(c, jobName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				c.IndentedJSON(200, batchv1.Job{})
				return
			}
			c.String(500, "get job error %v", err)
			return
		}
		conf, err := config.LoadUpgradeConfig(c, api.client, job.Labels[common.JfsUpgradeConfig])
		if err != nil {
			c.String(500, "get job error %v", err)
			return
		}
		total := 0
		for _, batch := range conf.Batches {
			total += len(batch)
		}

		pods, err := api.podSvc.ListBatchPods(c, conf)
		if err != nil {
			c.String(500, "list pods error %v", err)
			return
		}
		_, diffs, err := api.genPodDiffs(c, pods, false, false)
		if err != nil {
			c.String(500, "get pods diff configs error %v", err)
			return
		}
		pageSize, err := strconv.ParseUint(c.Query("pageSize"), 10, 64)
		if err != nil || pageSize == 0 {
			pageSize = uint64(len(diffs))
		}
		current, err := strconv.ParseUint(c.Query("current"), 10, 64)
		if err != nil || current == 0 {
			current = 1
		}
		diffPods := make([]PodDiff, 0)
		for i := (current - 1) * pageSize; i < current*pageSize && i < uint64(len(diffs)); i++ {
			diffPods = append(diffPods, diffs[i])
		}
		c.IndentedJSON(200, map[string]interface{}{
			"job":    job,
			"config": conf,
			"diffs":  diffPods,
			"total":  total,
		})
	}
}

func (api *API) updateUpgradeJob() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobName := c.Param("jobName")
		job, err := api.client.BatchV1().Jobs(config.Namespace).Get(c, jobName, metav1.GetOptions{})
		if err != nil {
			c.String(500, "get job error %v", err)
			return
		}
		type body struct {
			Action string `json:"action"`
		}
		action := &body{}
		if err := c.ShouldBindJSON(action); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		conf, err := config.LoadUpgradeConfig(c, api.client, job.Labels[common.JfsUpgradeConfig])
		if err != nil {
			c.String(500, "get job error %v", err)
			return
		}
		if !api.canDoAction(c, conf.Status, action.Action) {
			c.String(400, "can not %s job", action.Action)
			return
		}
		pod, err := api.getPodOfUpgradeJob(c, job)
		if err != nil {
			c.String(500, "get pod of job error %v", err)
			return
		}

		err = api.doActionInUpgradeJob(c, pod, action.Action)
		if err != nil {
			c.String(500, "do action in job error %v", err)
			return
		}

		c.IndentedJSON(200, map[string]string{
			"jobName": jobName,
		})
	}
}

func (api *API) deleteUpgradeJob() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobName := c.Param("jobName")
		job, err := api.client.BatchV1().Jobs(config.Namespace).Get(c, jobName, metav1.GetOptions{})
		if err != nil {
			c.String(500, "get job error %v", err)
			return
		}
		conf, err := config.LoadUpgradeConfig(c, api.client, job.Labels[common.JfsUpgradeConfig])
		if err != nil {
			c.String(500, "get job error %v", err)
			return
		}
		if !api.canDoAction(c, conf.Status, "delete") {
			c.String(400, "can not delete job")
			return
		}
		err = api.client.BatchV1().Jobs(config.Namespace).Delete(c, jobName, metav1.DeleteOptions{
			PropagationPolicy: util.ToPtr(metav1.DeletePropagationBackground),
		})
		if err != nil && !k8serrors.IsNotFound(err) {
			c.String(500, "delete job error %v", err)
			return
		}
	}
}

func (api *API) getUpgradeJobLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		jobName := c.Param("jobName")
		job, err := api.client.BatchV1().Jobs(config.Namespace).Get(c, jobName, metav1.GetOptions{})
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
				common.PodTypeKey:        common.JobTypeValue,
				common.JfsJobKind:        common.KindOfUpgrade,
				common.JfsUpgradeJobName: jobName,
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
			jobName := c.Param("jobName")
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
				job, err = api.client.BatchV1().Jobs(config.Namespace).Get(c, jobName, metav1.GetOptions{})
				if err == nil && job.DeletionTimestamp == nil {
					s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.PodTypeKey:        common.JobTypeValue,
							common.JfsJobKind:        common.KindOfUpgrade,
							common.JfsUpgradeJobName: jobName,
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
					_, _ = ws.Write([]byte(fmt.Sprintf("Upgrade timeout, job for upgrade is not ready, please check job [%s] in [%s] and try again later.", jobName, config.Namespace)))
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
			stream, err := req.Stream(c)
			if err != nil {
				fmt.Printf("err in stream: %s", err)
				return
			}
			wr := utils.NewLogPipe(c.Request.Context(), ws, stream)
			_, err = io.Copy(wr, wr)
			if err != nil {
				fmt.Printf("err in copy: %s", err)
				return
			}
		}).ServeHTTP(c.Writer, c.Request)
	}
}

func newUpgradeJob(jobName string) *batchv1.Job {
	sysNamespace := config.Namespace
	cmds := []string{"juicefs-csi-dashboard", "upgrade"}
	sa := "juicefs-csi-dashboard-sa"
	if os.Getenv("JUICEFS_CSI_DASHBOARD_SA") != "" {
		sa = os.Getenv("JUICEFS_CSI_DASHBOARD_SA")
	}
	configName := GenUpgradeConfig(jobName)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: sysNamespace,
			Labels: map[string]string{
				common.PodTypeKey:       common.JobTypeValue,
				common.JfsJobKind:       common.KindOfUpgrade,
				common.JfsUpgradeConfig: configName,
			},
		},
		Spec: batchv1.JobSpec{
			Parallelism:             util.ToPtr(int32(1)),
			Completions:             util.ToPtr(int32(1)),
			BackoffLimit:            util.ToPtr(int32(0)),
			TTLSecondsAfterFinished: util.ToPtr(int32(3600 * 24 * 7)), // automatically deleted after 7 day
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.PodTypeKey:        common.JobTypeValue,
						common.JfsJobKind:        common.KindOfUpgrade,
						common.JfsUpgradeJobName: jobName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "juicefs-upgrade",
						Image:   os.Getenv("DASHBOARD_IMAGE"),
						Command: cmds,
						Env: []corev1.EnvVar{
							{Name: "SYS_NAMESPACE", Value: sysNamespace},
							{Name: common.JfsUpgradeConfig, Value: configName},
						},
					}},
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: sa,
				},
			},
		},
	}
}

type PodDiff struct {
	Pod        corev1.Pod           `json:"pod"`
	OldConfig  config.MountPodPatch `json:"oldConfig"`
	OldSetting *config.JfsSetting   `json:"oldSetting,omitempty"`
	NewConfig  config.MountPodPatch `json:"newConfig"`
	NewSetting *config.JfsSetting   `json:"newSetting,omitempty"`
}

type ListDiffPodResult struct {
	Total int       `json:"total"`
	Pods  []PodDiff `json:"pods"`
}

// genPodDiffs return mount pods with diff configs
// mountPods: pods need to get diff configs
// shouldDiff: should pass the pods which have no diff config
func (api *API) genPodDiffs(ctx context.Context, mountPods []corev1.Pod, shouldDiff, debug bool) ([]corev1.Pod, []PodDiff, error) {
	// load config
	if err := config.LoadFromConfigMap(ctx, api.client); err != nil {
		return nil, nil, err
	}

	// get pvc、pv、secret
	pvs, err := api.pvSvc.ListAllPVs(ctx)
	if err != nil {
		return nil, nil, err
	}
	pvcs, err := api.pvcSvc.ListAllPVCs(ctx, pvs)
	if err != nil {
		return nil, nil, err
	}

	secrets, err := api.secretSvc.ListAllSecrets(ctx)
	if err != nil {
		return nil, nil, err
	}
	pvMap := make(map[string]*corev1.PersistentVolume)
	pvcMap := make(map[string]*corev1.PersistentVolumeClaim)
	secretMap := make(map[string]*corev1.Secret)
	custSecretMap := make(map[string]*corev1.Secret)
	for _, pv := range pvs {
		pvMap[pv.Name] = &pv
	}
	for _, pvc := range pvcs {
		pvc2 := pvc
		pvcMap[pvc.Spec.VolumeName] = &pvc2
	}
	for _, secret := range secrets {
		secret2 := secret
		uniqueId := getUniqueIdFromSecretName(secret2.Name)
		if uniqueId != "" {
			secretMap[uniqueId] = &secret2
		}
		custSecretMap[secret2.Name] = &secret2
	}

	var needUpdatePods []corev1.Pod
	var podDiffs []PodDiff
	for _, pod := range mountPods {
		po := pod
		pv := pvMap[po.Annotations[common.UniqueId]]
		var custSecret *corev1.Secret
		if pv != nil && pv.Spec.CSI != nil && pv.Spec.CSI.NodePublishSecretRef != nil {
			custSecret = custSecretMap[pv.Spec.CSI.NodePublishSecretRef.Name]
		}
		diff, err := DiffConfig(&po, pv, pvcMap[po.Annotations[common.UniqueId]], secretMap[po.Annotations[common.UniqueId]], custSecret)
		if err != nil {
			return nil, nil, err
		}
		if !diff && shouldDiff {
			// no diff config and should diff, skip
			continue
		}
		oldConfig, oldSetting, newConfig, newSetting, err := config.GetDiff(&po, pvcMap[po.Annotations[common.UniqueId]], pv, secretMap[po.Annotations[common.UniqueId]], custSecret)
		if err != nil {
			return nil, nil, err
		}
		needUpdatePods = append(needUpdatePods, po)
		pd := PodDiff{
			Pod:       po,
			OldConfig: *oldConfig,
			NewConfig: *newConfig,
		}
		if debug {
			pd.OldSetting = oldSetting
			pd.NewSetting = newSetting
		}
		podDiffs = append(podDiffs, pd)
	}
	return needUpdatePods, podDiffs, nil
}

func GenUpgradeJobName() string {
	return fmt.Sprintf("jfs-upgrade-job-%s", util.RandStringRunes(6))
}

func GenUpgradeConfig(jobName string) string {
	return fmt.Sprintf("%s-config", jobName)
}

func (api *API) getAllUpgradeConfig(ctx context.Context) (map[string]*config.BatchConfig, error) {
	var (
		cmList  *corev1.ConfigMapList
		configs = make(map[string]*config.BatchConfig)
		err     error
	)
	s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey: common.ConfigTypeValue,
		},
	})
	cmList, err = api.client.CoreV1().ConfigMaps(config.Namespace).List(ctx, metav1.ListOptions{LabelSelector: s.String()})
	if err != nil {
		return nil, err
	}
	for _, cm := range cmList.Items {
		cfg, err := config.LoadBatchConfig(&cm)
		if err != nil {
			return nil, err
		}
		configs[cm.Name] = cfg
	}
	return configs, nil
}

func (api *API) getPodOfUpgradeJob(c context.Context, job *batchv1.Job) (*corev1.Pod, error) {
	if job == nil {
		return nil, nil
	}
	s, _ := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.PodTypeKey:        common.JobTypeValue,
			common.JfsJobKind:        common.KindOfUpgrade,
			common.JfsUpgradeJobName: job.Name,
		},
	})
	podList, err := api.client.CoreV1().Pods(job.Namespace).List(c, metav1.ListOptions{LabelSelector: s.String()})
	if err == nil && len(podList.Items) != 0 {
		return &podList.Items[0], nil
	}
	return nil, nil
}

func (api *API) canDoAction(ctx context.Context, status config.UpgradeStatus, action string) bool {
	switch action {
	case "stop":
		return status != config.Fail &&
			status != config.Success &&
			status != config.Stop
	case "resume":
		return status == config.Pause
	case "pause":
		return status != config.Stop &&
			status != config.Pause &&
			status != config.Fail &&
			status != config.Success
	case "delete":
		return status == config.Fail || status == config.Success || status == config.Stop || status == config.Pause
	}
	return false
}

func (api *API) doActionInUpgradeJob(ctx context.Context, pod *corev1.Pod, action string) error {
	sig := "-SIGUSR1"
	if action == "stop" {
		sig = "-SIGTERM"
	}
	req := api.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   []string{"kill", sig, "1"},
		Container: "juicefs-upgrade",
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(api.kubeconfig, "POST", req.URL())
	if err != nil {
		return err
	}
	var sout, serr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &sout,
		Stderr: &serr,
		Tty:    false,
	})
	if err != nil {
		batchLog.Info("kill -SIGUSR1", "pod", pod.Name, "stdout", strings.TrimSpace(sout.String()), "stderr", strings.TrimSpace(serr.String()), "error", err)
		return err
	}

	return err
}
