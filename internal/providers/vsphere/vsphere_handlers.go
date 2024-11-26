package vsphere

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
)

func (v *VSphereClient) GetPodsHandler(c *gin.Context) {
    _, span := tracer.Start(c.Request.Context(), "GET /api/v1/view/pods")
    defer span.End()

    username := sessions.Default(c).Get("id")
    span.SetAttributes(attribute.String("username", username.(string)))

    pods, err := vSphereGetPods(username.(string))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"message": "Error getting pods"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"pods": pods})
}

func (v *VSphereClient) DeletePodHandler(c *gin.Context) {
    ctx, span := tracer.Start(c.Request.Context(), "DELETE /api/v1/pod/delete")
    defer span.End()

    podId := c.Param("podId")
    username := sessions.Default(c).Get("id")
    podOwner := strings.Split(podId, "_")
    podOwner = podOwner[len(podOwner)-1:]

    span.SetAttributes(attribute.String("deleted-pod", podId))

    if strings.ToLower(username.(string)) != strings.ToLower(podOwner[0]) {
        c.JSON(http.StatusUnauthorized, gin.H{"message": "Unauthorized"})
        return
    }

    err := DestroyResources(ctx, podId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deleted successfully!"})
}

func (v *VSphereClient) GetPresetTemplatesHandler(c *gin.Context) {
    _, span := tracer.Start(c.Request.Context(), "GET /api/v1/view/templates")
    defer span.End()

    isAdmin := sessions.Default(c).Get("isAdmin").(bool)
    templates, err := v.vSphereGetPresetTemplates(isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (v *VSphereClient) GetTemplateVMsHandler(c *gin.Context) {
    _, span := tracer.Start(c.Request.Context(), "GET /api/v1/view/template/vms")
    defer span.End()

    templates, err := vSphereGetCustomTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

func (v *VSphereClient) CloneFromTemplateHandler(c *gin.Context) {
    ctx, span := tracer.Start(c.Request.Context(), "POST /api/v1/pod/clone/template")
    defer span.End()

    var jsonData map[string]interface{} // cheaty solution to avoid form struct xd
	err := c.ShouldBindJSON(&jsonData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	template := jsonData["template"].(string)
    username := sessions.Default(c).Get("id").(string)

    span.SetAttributes(attribute.String("template", template))
    span.SetAttributes(attribute.String("username", username))

	fmt.Printf("User %s is cloning template %s\n", username, template)
	err = v.vSphereTemplateClone(ctx, template, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod deployed successfully!"})
}

func (v *VSphereClient) CloneCustomPodHandler(c *gin.Context) {
    ctx, span := tracer.Start(c.Request.Context(), "POST /api/v1/pod/clone/custom")
    defer span.End()

    username := sessions.Default(c).Get("id").(string)

	var form struct {
		Name       string   `json:"name"`
		Nat        bool     `json:"nat"`
		Vmstoclone []string `json:"vmstoclone"`
	}

    span.SetAttributes(attribute.String("username", username))
    span.SetAttributes(attribute.String("pod-name", form.Name))
    span.SetAttributes(attribute.Bool("nat", form.Nat))
    span.SetAttributes(attribute.StringSlice("vms-to-clone", form.Vmstoclone))

	err := c.ShouldBindJSON(&form)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(form.Vmstoclone) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many VMs in custom pod"})
		return
	}

	fmt.Printf("User %s is cloning custom pod %s\n", username, form.Name)
	err = v.vSphereCustomClone(ctx, form.Name, form.Vmstoclone, form.Nat, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Pod deployed successfully!"})
}

func (v *VSphereClient) RefreshTemplatesHandler(c *gin.Context) {
    ctx, span := tracer.Start(c.Request.Context(), "POST /api/v1/admin/templates/refresh")
    defer span.End()

    err := LoadTemplates(ctx)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Templates refreshed successfully!"})
}

func (v *VSphereClient) BulkClonePodsHandler(c *gin.Context) {
    ctx, span := tracer.Start(c.Request.Context(), "POST /api/v1/admin/pod/clone/bulk")
    defer span.End()

    username := sessions.Default(c).Get("id").(string)


    var form struct {
        Template string `json:"template"`
        Names []string `json:"names"`
    }

    span.SetAttributes(attribute.String("username", username))
    span.SetAttributes(attribute.Int("num-clones", len(form.Names)))
    span.SetAttributes(attribute.String("template", form.Template))

    err := c.ShouldBindJSON(&form)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    fmt.Printf("User %s is cloning %d pods\n", username, len(form.Names))
    eg := errgroup.Group{}
    for _, name := range form.Names {
        if name == "" {
            continue
        }
        eg.Go(func() error {
            return v.vSphereTemplateClone(ctx, form.Template, name)
        },)
    }

    if err := eg.Wait(); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Pods deployed successfully!"})
}

func (v *VSphereClient) BulkDeletePodsHandler(c *gin.Context) {
    ctx, span := tracer.Start(c.Request.Context(), "DELETE /api/v1/admin/pod/delete/bulk")
    defer span.End()

    var form struct {
        Filters []string `json:"filters"`
    }

    span.SetAttributes(attribute.StringSlice("num-pods", form.Filters))

    err := c.ShouldBindJSON(&form)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    failed, err := bulkDeletePods(ctx, form.Filters)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if len(failed) > 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to delete pods", "failed": failed})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Pods deleted successfully!"})
}

func (v *VSphereClient) BulkRevertPodHandler(c *gin.Context) {
    var form struct {
		Filters  []string `json:"filters"`
		Snapshot string   `json:"snapshot"`
	}

	err := c.ShouldBindJSON(&form)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

	failed, err := bulkRevertPods(form.Filters, form.Snapshot)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(failed) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to revert pods", "failed": failed})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pods reverted successfully!"})
}

func (v *VSphereClient) BulkPowerPodHandler(c *gin.Context) {
    var form struct {
        Filters []string `json:"filters"`
        Power   bool   `json:"power"`
    }

    err := c.ShouldBindJSON(&form)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    failed, err := bulkPowerPods(form.Filters, form.Power)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if len(failed) > 0 {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to power pods", "failed": failed})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Pods powered successfully!"})
}
