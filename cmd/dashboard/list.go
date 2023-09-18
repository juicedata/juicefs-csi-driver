package main

import "github.com/gin-gonic/gin"

func (api *dashboardApi) listPod() gin.HandlerFunc {
	return func(c *gin.Context) {
		pods, err := api.k8sClient.ListPod(c, "kube-system", nil, nil)
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(200, pods)
	}
}
