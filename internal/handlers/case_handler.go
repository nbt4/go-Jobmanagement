package handlers

import (
	"log"
	"net/http"
	"strconv"

	"go-barcode-webapp/internal/models"
	"go-barcode-webapp/internal/repository"

	"github.com/gin-gonic/gin"
)

type CaseHandler struct {
	caseRepo   *repository.CaseRepository
	deviceRepo *repository.DeviceRepository
}

func NewCaseHandler(caseRepo *repository.CaseRepository, deviceRepo *repository.DeviceRepository) *CaseHandler {
	return &CaseHandler{
		caseRepo:   caseRepo,
		deviceRepo: deviceRepo,
	}
}

// Web interface handlers
func (h *CaseHandler) ListCases(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	params := &models.FilterParams{}
	if err := c.ShouldBindQuery(params); err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	cases, err := h.caseRepo.List(params)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	c.HTML(http.StatusOK, "cases_list.html", gin.H{
		"title": "Cases",
		"cases": cases,
		"params": params,
		"user": user,
	})
}

func (h *CaseHandler) CaseManagement(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	params := &models.FilterParams{}
	cases, err := h.caseRepo.List(params)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	// Get available devices for case management
	devices, err := h.deviceRepo.GetAvailableDevicesForCaseManagement()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	c.HTML(http.StatusOK, "case_management_simple.html", gin.H{
		"title": "Case Management",
		"cases": cases,
		"devices": devices,
		"user": user,
	})
}


func (h *CaseHandler) NewCaseForm(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	c.HTML(http.StatusOK, "case_form.html", gin.H{
		"title": "New Case",
		"case":  &models.Case{},
		"user": user,
	})
}

func (h *CaseHandler) CreateCase(c *gin.Context) {
	name := c.PostForm("name")
	description := c.PostForm("description")
	status := c.PostForm("status")
	if status == "" {
		status = "free"
	}

	// Parse optional numeric fields
	var weight, width, height, depth *float64
	if weightStr := c.PostForm("weight"); weightStr != "" {
		if w, err := strconv.ParseFloat(weightStr, 64); err == nil {
			weight = &w
		}
	}
	if widthStr := c.PostForm("width"); widthStr != "" {
		if w, err := strconv.ParseFloat(widthStr, 64); err == nil {
			width = &w
		}
	}
	if heightStr := c.PostForm("height"); heightStr != "" {
		if h, err := strconv.ParseFloat(heightStr, 64); err == nil {
			height = &h
		}
	}
	if depthStr := c.PostForm("depth"); depthStr != "" {
		if d, err := strconv.ParseFloat(depthStr, 64); err == nil {
			depth = &d
		}
	}

	case_ := models.Case{
		Name:        name,
		Description: &description,
		Weight:      weight,
		Width:       width,
		Height:      height,
		Depth:       depth,
		Status:      status,
	}

	if err := h.caseRepo.Create(&case_); err != nil {
		user, _ := GetCurrentUser(c)
		c.HTML(http.StatusInternalServerError, "case_form.html", gin.H{
			"title": "New Case",
			"case":  &case_,
			"error": err.Error(),
			"user": user,
		})
		return
	}

	c.Redirect(http.StatusFound, "/cases")
}

func (h *CaseHandler) GetCase(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid case ID", "user": user})
		return
	}

	case_, err := h.caseRepo.GetByID(uint(caseID))
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Case not found", "user": user})
		return
	}

	c.HTML(http.StatusOK, "case_detail.html", gin.H{
		"case": case_,
		"user": user,
	})
}

func (h *CaseHandler) EditCaseForm(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid case ID", "user": user})
		return
	}

	case_, err := h.caseRepo.GetByID(uint(caseID))
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Case not found", "user": user})
		return
	}

	c.HTML(http.StatusOK, "case_form.html", gin.H{
		"title": "Edit Case",
		"case":  case_,
		"user": user,
	})
}

func (h *CaseHandler) UpdateCase(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid case ID", "user": user})
		return
	}

	name := c.PostForm("name")
	description := c.PostForm("description")
	status := c.PostForm("status")

	// Parse optional numeric fields
	var weight, width, height, depth *float64
	if weightStr := c.PostForm("weight"); weightStr != "" {
		if w, err := strconv.ParseFloat(weightStr, 64); err == nil {
			weight = &w
		}
	}
	if widthStr := c.PostForm("width"); widthStr != "" {
		if w, err := strconv.ParseFloat(widthStr, 64); err == nil {
			width = &w
		}
	}
	if heightStr := c.PostForm("height"); heightStr != "" {
		if h, err := strconv.ParseFloat(heightStr, 64); err == nil {
			height = &h
		}
	}
	if depthStr := c.PostForm("depth"); depthStr != "" {
		if d, err := strconv.ParseFloat(depthStr, 64); err == nil {
			depth = &d
		}
	}

	case_ := models.Case{
		CaseID:      uint(caseID),
		Name:        name,
		Description: &description,
		Weight:      weight,
		Width:       width,
		Height:      height,
		Depth:       depth,
		Status:      status,
	}

	if err := h.caseRepo.Update(&case_); err != nil {
		c.HTML(http.StatusInternalServerError, "case_form.html", gin.H{
			"title": "Edit Case",
			"case":  &case_,
			"error": err.Error(),
			"user": user,
		})
		return
	}

	c.Redirect(http.StatusFound, "/cases")
}

func (h *CaseHandler) DeleteCase(c *gin.Context) {
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid case ID"})
		return
	}

	if err := h.caseRepo.Delete(uint(caseID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Case deleted successfully"})
}

// Device mapping handlers
func (h *CaseHandler) CaseDeviceMapping(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid case ID", "user": user})
		return
	}

	case_, err := h.caseRepo.GetByID(uint(caseID))
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Case not found", "user": user})
		return
	}

	deviceCases, err := h.caseRepo.GetDevicesInCase(uint(caseID))
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	c.HTML(http.StatusOK, "case_device_mapping.html", gin.H{
		"title":       "Case Device Mapping",
		"case":        case_,
		"deviceCases": deviceCases,
		"user": user,
	})
}

func (h *CaseHandler) ScanDeviceToCase(c *gin.Context) {
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid case ID"})
		return
	}

	var request struct {
		DeviceID string `json:"device_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if device exists
	device, err := h.deviceRepo.GetByID(request.DeviceID)
	if err != nil {
		// Try by serial number
		device, err = h.deviceRepo.GetBySerialNo(request.DeviceID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
			return
		}
	}

	// Check if device is already in a case
	isInCase, err := h.caseRepo.IsDeviceInAnyCase(device.DeviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if isInCase {
		c.JSON(http.StatusConflict, gin.H{"error": "Device is already in a case"})
		return
	}

	// Add device to case
	err = h.caseRepo.AddDeviceToCase(uint(caseID), device.DeviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Device added to case successfully",
		"device":  device,
	})
}

func (h *CaseHandler) RemoveDeviceFromCase(c *gin.Context) {
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid case ID"})
		return
	}

	deviceID := c.Param("deviceId")

	err = h.caseRepo.RemoveDeviceFromCase(uint(caseID), deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device removed from case successfully"})
}

// API handlers
func (h *CaseHandler) ListCasesAPI(c *gin.Context) {
	params := &models.FilterParams{}
	if err := c.ShouldBindQuery(params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cases, err := h.caseRepo.List(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cases": cases})
}

func (h *CaseHandler) CreateCaseAPI(c *gin.Context) {
	var case_ models.Case
	if err := c.ShouldBindJSON(&case_); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.caseRepo.Create(&case_); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, case_)
}

func (h *CaseHandler) GetCaseAPI(c *gin.Context) {
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid case ID"})
		return
	}

	case_, err := h.caseRepo.GetByID(uint(caseID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Case not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"case": case_})
}

func (h *CaseHandler) UpdateCaseAPI(c *gin.Context) {
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid case ID"})
		return
	}

	var case_ models.Case
	if err := c.ShouldBindJSON(&case_); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	case_.CaseID = uint(caseID)
	if err := h.caseRepo.Update(&case_); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, case_)
}

func (h *CaseHandler) DeleteCaseAPI(c *gin.Context) {
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid case ID"})
		return
	}

	if err := h.caseRepo.Delete(uint(caseID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Case deleted successfully"})
}

// GetCaseDevicesAPI returns devices in a case as JSON
func (h *CaseHandler) GetCaseDevicesAPI(c *gin.Context) {
	caseIDStr := c.Param("id")
	log.Printf("GetCaseDevicesAPI: Getting devices for case ID: %s", caseIDStr)
	
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		log.Printf("GetCaseDevicesAPI: Invalid case ID: %s, error: %v", caseIDStr, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid case ID"})
		return
	}

	deviceCases, err := h.caseRepo.GetDevicesInCase(uint(caseID))
	if err != nil {
		log.Printf("GetCaseDevicesAPI: Database error for case %d: %v", caseID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Ensure we always return an array, never null
	if deviceCases == nil {
		deviceCases = []models.DeviceCase{}
	}

	log.Printf("GetCaseDevicesAPI: Found %d devices for case %d", len(deviceCases), caseID)
	c.JSON(http.StatusOK, gin.H{"devices": deviceCases})
}