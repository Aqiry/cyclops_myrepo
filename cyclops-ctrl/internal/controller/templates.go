package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	cyclopsv1alpha1 "github.com/cyclops-ui/cycops-ctrl/api/v1alpha1"
	"github.com/cyclops-ui/cycops-ctrl/internal/cluster/k8sclient"
	"github.com/cyclops-ui/cycops-ctrl/internal/mapper"
	"github.com/cyclops-ui/cycops-ctrl/internal/models"
	"github.com/cyclops-ui/cycops-ctrl/internal/models/dto"
	"github.com/cyclops-ui/cycops-ctrl/internal/storage/templates"
	"github.com/cyclops-ui/cycops-ctrl/internal/template"
)

type Templates struct {
	templates        *templates.Storage
	templatesRepo    *template.Repo
	kubernetesClient *k8sclient.KubernetesClient
}

func NewTemplatesController(
	templates *templates.Storage,
	templatesRepo *template.Repo,
	kubernetes *k8sclient.KubernetesClient,
) *Templates {
	return &Templates{
		templatesRepo:    templatesRepo,
		templates:        templates,
		kubernetesClient: kubernetes,
	}
}

func (c *Templates) StoreConfiguration(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	var request models.Template
	if err := ctx.BindJSON(&request); err != nil {
		fmt.Println("error binding request", request)
		ctx.Status(http.StatusBadRequest)
		return
	}

	// TODO this needs to be moved to some other layer
	request.Created = time.Now().Format(time.RFC822)
	request.Edited = time.Now().Format(time.RFC822)

	request.Version = semantic(request.Version)

	if err := c.templates.StoreConfig(request); err != nil {
		fmt.Println(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.Status(http.StatusOK)
}

// TODO kaj je ovo
func semantic(current string) string {
	if len(current) == 0 {
		return "v1"
	}

	version, _ := strconv.Atoi(current[1:])

	version++

	return fmt.Sprintf("v%d", version)
}

func (c *Templates) GetConfiguration(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	name := ctx.Param("name")
	version := ctx.Query("version")

	configuration, err := c.templates.GetConfigByVersion(name, version)
	if err != nil {
		fmt.Println(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	modules, err := c.kubernetesClient.ListModules()
	if err != nil {
		fmt.Println(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	related := make([]cyclopsv1alpha1.Module, 0)
	for _, module := range modules {
		if name != module.Spec.TemplateRef.Path {
			continue
		}

		related = append(related, module)
	}

	configuration.Modules = mapper.ModuleListToDTO(related)

	ctx.JSON(http.StatusOK, configuration)
}

func (c *Templates) GetConfigurationsDetails(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	templates, err := c.templates.ListConfigLatest()
	if err != nil {
		fmt.Println(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	templates = mapper.MapConfigDetails(templates)

	ctx.JSON(http.StatusOK, templates)
}

func (c *Templates) GetConfigurationsVersions(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	versions, err := c.templates.GetConfigurationVersions(ctx.Param("name"))
	if err != nil {
		fmt.Println(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, versions)
}

func (c *Templates) GetTemplate(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	repo := ctx.Query("repo")
	path := ctx.Query("path")
	commit := ctx.Query("commit")

	if repo == "" {
		ctx.String(http.StatusBadRequest, "set repo field")
		return
	}

	t, err := c.templatesRepo.GetTemplate(repo, path, commit)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusBadRequest, dto.NewError("Error loading template", err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, t)
}

func (c *Templates) GetTemplateInitialValues(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	repo := ctx.Query("repo")
	path := ctx.Query("path")
	commit := ctx.Query("commit")

	if repo == "" {
		ctx.JSON(http.StatusBadRequest, dto.NewError("Specify repo field", "Repo not specified"))
		return
	}

	initial, err := c.templatesRepo.GetTemplateInitialValues(repo, path, commit)
	if err != nil {
		fmt.Println(err)
		ctx.JSON(http.StatusBadRequest, dto.NewError("Error loading template initial values", err.Error()))
		return
	}

	ctx.Data(http.StatusOK, gin.MIMEJSON, initial)
}

func (c *Templates) ListTemplatesStore(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	store, err := c.kubernetesClient.ListTemplateStore()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.NewError("Error fetching templates store", err.Error()))
		return
	}

	storeDTO := mapper.TemplateStoreListToDTO(store)

	ctx.JSON(http.StatusOK, storeDTO)
}

func (c *Templates) CreateTemplatesStore(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	var templateStore *dto.TemplateStore
	if err := ctx.ShouldBind(&templateStore); err != nil {
		fmt.Println("error binding request", templateStore)
		ctx.JSON(http.StatusBadRequest, dto.NewError("Error binding request", err.Error()))
		return
	}

	k8sTemplateStore := mapper.DTOToTemplateStore(*templateStore)

	if err := c.kubernetesClient.CreateTemplateStore(k8sTemplateStore); err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.NewError("Error creating module", err.Error()))
		return
	}

	ctx.Status(http.StatusCreated)
}

func (c *Templates) EditTemplatesStore(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	var templateStore *dto.TemplateStore
	if err := ctx.ShouldBind(&templateStore); err != nil {
		fmt.Println("error binding request", templateStore)
		ctx.JSON(http.StatusBadRequest, dto.NewError("Error binding request", err.Error()))
		return
	}

	templateStore.Name = ctx.Param("name")

	k8sTemplateStore := mapper.DTOToTemplateStore(*templateStore)

	if err := c.kubernetesClient.UpdateTemplateStore(k8sTemplateStore); err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.NewError("Error creating module", err.Error()))
		return
	}

	ctx.Status(http.StatusCreated)
}

func (c *Templates) DeleteTemplatesStore(ctx *gin.Context) {
	ctx.Header("Access-Control-Allow-Origin", "*")

	templateRefName := ctx.Param("name")

	if err := c.kubernetesClient.DeleteTemplateStore(templateRefName); err != nil {
		ctx.JSON(http.StatusInternalServerError, dto.NewError("Error deleting module", err.Error()))
		return
	}

	ctx.Status(http.StatusOK)
}
