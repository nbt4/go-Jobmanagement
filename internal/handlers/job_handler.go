package handlers

import (
	"net/http"
	"strconv"
	"time"

	"go-barcode-webapp/internal/models"
	"go-barcode-webapp/internal/repository"

	"github.com/gin-gonic/gin"
)

type JobHandler struct {
	jobRepo         *repository.JobRepository
	deviceRepo      *repository.DeviceRepository
	customerRepo    *repository.CustomerRepository
	statusRepo      *repository.StatusRepository
	jobCategoryRepo *repository.JobCategoryRepository
}

func NewJobHandler(jobRepo *repository.JobRepository, deviceRepo *repository.DeviceRepository, customerRepo *repository.CustomerRepository, statusRepo *repository.StatusRepository, jobCategoryRepo *repository.JobCategoryRepository) *JobHandler {
	return &JobHandler{
		jobRepo:         jobRepo,
		deviceRepo:      deviceRepo,
		customerRepo:    customerRepo,
		statusRepo:      statusRepo,
		jobCategoryRepo: jobCategoryRepo,
	}
}

// Web interface handlers
func (h *JobHandler) ListJobs(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	params := &models.FilterParams{}
	if err := c.ShouldBindQuery(params); err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	jobs, err := h.jobRepo.List(params)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	c.HTML(http.StatusOK, "jobs.html", gin.H{
		"title":  "Jobs",
		"jobs":   jobs,
		"params": params,
		"user":   user,
	})
}

func (h *JobHandler) NewJobForm(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	customers, err := h.customerRepo.List(&models.FilterParams{})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	statuses, err := h.statusRepo.List()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	jobCategories, err := h.jobCategoryRepo.List()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	c.HTML(http.StatusOK, "job_form.html", gin.H{
		"title":        "New Job",
		"job":          &models.Job{},
		"customers":    customers,
		"statuses":     statuses,
		"jobCategories": jobCategories,
		"user":         user,
	})
}

func (h *JobHandler) CreateJob(c *gin.Context) {
	customerID, _ := strconv.ParseUint(c.PostForm("customer_id"), 10, 32)
	statusID, _ := strconv.ParseUint(c.PostForm("status_id"), 10, 32)

	var startDate, endDate *time.Time
	if startDateStr := c.PostForm("start_date"); startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = &parsed
		}
	}
	if endDateStr := c.PostForm("end_date"); endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = &parsed
		}
	}

	description := c.PostForm("description")
	discountType := c.PostForm("discount_type")
	if discountType == "" {
		discountType = "amount" // default
	}
	
	job := models.Job{
		CustomerID:   uint(customerID),
		StatusID:     uint(statusID),
		Description:  &description,
		StartDate:    startDate,
		EndDate:      endDate,
		DiscountType: discountType,
	}

	if jobCategoryIDStr := c.PostForm("job_category_id"); jobCategoryIDStr != "" {
		if jobCategoryID, err := strconv.ParseUint(jobCategoryIDStr, 10, 32); err == nil {
			jobCatID := uint(jobCategoryID)
			job.JobCategoryID = &jobCatID
		}
	}

	if revenueStr := c.PostForm("revenue"); revenueStr != "" {
		if revenue, err := strconv.ParseFloat(revenueStr, 64); err == nil {
			job.Revenue = revenue
		}
	}

	if discountStr := c.PostForm("discount"); discountStr != "" {
		if discount, err := strconv.ParseFloat(discountStr, 64); err == nil {
			job.Discount = discount
		}
	}

	if err := h.jobRepo.Create(&job); err != nil {
		user, _ := GetCurrentUser(c)
		customers, _ := h.customerRepo.List(&models.FilterParams{})
		statuses, _ := h.statusRepo.List()
		jobCategories, _ := h.jobCategoryRepo.List()
		c.HTML(http.StatusInternalServerError, "job_form.html", gin.H{
			"title":        "New Job",
			"job":          &job,
			"customers":    customers,
			"statuses":     statuses,
			"jobCategories": jobCategories,
			"error":        err.Error(),
			"user":         user,
		})
		return
	}

	c.Redirect(http.StatusFound, "/jobs")
}

func (h *JobHandler) GetJob(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid job ID", "user": user})
		return
	}

	job, err := h.jobRepo.GetByID(uint(id))
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Job not found", "user": user})
		return
	}

	jobDevices, err := h.jobRepo.GetJobDevices(uint(id))
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	// Group devices by product and calculate pricing
	productGroups := make(map[string]*ProductGroup)
	totalDevices := len(jobDevices)
	totalValue := 0.0

	for _, jd := range jobDevices {
		if jd.Device.Product == nil {
			continue
		}

		productName := jd.Device.Product.Name
		if _, exists := productGroups[productName]; !exists {
			productGroups[productName] = &ProductGroup{
				Product: jd.Device.Product,
				Devices: []models.JobDevice{},
			}
		}

		// Calculate effective price (custom price if set, otherwise default product price)
		var effectivePrice float64
		if jd.CustomPrice != nil && *jd.CustomPrice > 0 {
			effectivePrice = *jd.CustomPrice
		} else if jd.Device.Product.ItemCostPerDay != nil {
			effectivePrice = *jd.Device.Product.ItemCostPerDay
		}

		// Create a copy of the job device with calculated price for display
		jdCopy := jd
		jdCopy.CustomPrice = &effectivePrice

		productGroups[productName].Devices = append(productGroups[productName].Devices, jdCopy)
		productGroups[productName].TotalValue += effectivePrice
		totalValue += effectivePrice
	}

	c.HTML(http.StatusOK, "job_detail.html", gin.H{
		"job":           job,
		"jobDevices":    jobDevices,
		"productGroups": productGroups,
		"totalDevices":  totalDevices,
		"totalValue":    totalValue,
		"user":          user,
	})
}

func (h *JobHandler) EditJobForm(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid job ID", "user": user})
		return
	}

	job, err := h.jobRepo.GetByID(uint(id))
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Job not found", "user": user})
		return
	}

	customers, err := h.customerRepo.List(&models.FilterParams{})
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	statuses, err := h.statusRepo.List()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	jobCategories, err := h.jobCategoryRepo.List()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error(), "user": user})
		return
	}

	c.HTML(http.StatusOK, "job_form.html", gin.H{
		"title":        "Edit Job",
		"job":          job,
		"customers":    customers,
		"statuses":     statuses,
		"jobCategories": jobCategories,
		"user":         user,
	})
}

func (h *JobHandler) UpdateJob(c *gin.Context) {
	user, _ := GetCurrentUser(c)
	
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Invalid job ID", "user": user})
		return
	}

	// Load existing job first
	job, err := h.jobRepo.GetByID(uint(id))
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Job not found", "user": user})
		return
	}

	// Update fields from form
	customerID, _ := strconv.ParseUint(c.PostForm("customer_id"), 10, 32)
	statusID, _ := strconv.ParseUint(c.PostForm("status_id"), 10, 32)
	job.CustomerID = uint(customerID)
	job.StatusID = uint(statusID)

	var startDate, endDate *time.Time
	if startDateStr := c.PostForm("start_date"); startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = &parsed
		}
	}
	if endDateStr := c.PostForm("end_date"); endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = &parsed
		}
	}
	job.StartDate = startDate
	job.EndDate = endDate

	description := c.PostForm("description")
	job.Description = &description
	
	discountType := c.PostForm("discount_type")
	if discountType == "" {
		discountType = "amount" // default
	}
	job.DiscountType = discountType

	if jobCategoryIDStr := c.PostForm("job_category_id"); jobCategoryIDStr != "" {
		if jobCategoryID, err := strconv.ParseUint(jobCategoryIDStr, 10, 32); err == nil {
			jobCatID := uint(jobCategoryID)
			job.JobCategoryID = &jobCatID
		}
	}

	if revenueStr := c.PostForm("revenue"); revenueStr != "" {
		if revenue, err := strconv.ParseFloat(revenueStr, 64); err == nil {
			job.Revenue = revenue
		}
	}

	if discountStr := c.PostForm("discount"); discountStr != "" {
		if discount, err := strconv.ParseFloat(discountStr, 64); err == nil {
			job.Discount = discount
		}
	}

	if err := h.jobRepo.Update(job); err != nil {
		customers, _ := h.customerRepo.List(&models.FilterParams{})
		statuses, _ := h.statusRepo.List()
		jobCategories, _ := h.jobCategoryRepo.List()
		c.HTML(http.StatusInternalServerError, "job_form.html", gin.H{
			"title":        "Edit Job",
			"job":          job,
			"customers":    customers,
			"statuses":     statuses,
			"jobCategories": jobCategories,
			"error":        err.Error(),
			"user":         user,
		})
		return
	}

	// Only recalculate revenue automatically if no manual revenue was provided
	// This preserves manual revenue entries while still updating when dates change
	if c.PostForm("revenue") == "" {
		h.jobRepo.CalculateAndUpdateRevenue(uint(id))
	} else {
		// If manual revenue was provided, still calculate final_revenue based on discount
		h.jobRepo.UpdateFinalRevenue(uint(id))
	}

	c.Redirect(http.StatusFound, "/jobs")
}

func (h *JobHandler) DeleteJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	if err := h.jobRepo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job deleted successfully"})
}

func (h *JobHandler) GetJobDevices(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	jobDevices, err := h.jobRepo.GetJobDevices(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"devices": jobDevices})
}

func (h *JobHandler) AssignDevice(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	deviceID := c.PostForm("device_id")

	price, _ := strconv.ParseFloat(c.PostForm("price"), 64)

	if err := h.jobRepo.AssignDevice(uint(jobID), deviceID, price); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device assigned successfully"})
}

func (h *JobHandler) RemoveDevice(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	deviceID := c.Param("deviceId")

	if err := h.jobRepo.RemoveDevice(uint(jobID), deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device removed successfully"})
}

func (h *JobHandler) BulkScanDevices(c *gin.Context) {
	var request models.BulkScanRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := h.jobRepo.BulkAssignDevices(request.JobID, request.DeviceIDs, request.Price)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}

// API handlers
func (h *JobHandler) ListJobsAPI(c *gin.Context) {
	params := &models.FilterParams{}
	if err := c.ShouldBindQuery(params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	jobs, err := h.jobRepo.List(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

func (h *JobHandler) CreateJobAPI(c *gin.Context) {
	var job models.Job
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.jobRepo.Create(&job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, job)
}

func (h *JobHandler) GetJobAPI(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	job, err := h.jobRepo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *JobHandler) UpdateJobAPI(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	var job models.Job
	if err := c.ShouldBindJSON(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job.JobID = uint(id)
	if err := h.jobRepo.Update(&job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *JobHandler) DeleteJobAPI(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	if err := h.jobRepo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Job deleted successfully"})
}

func (h *JobHandler) AssignDeviceAPI(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	deviceID := c.Param("deviceId")

	var request struct {
		Price float64 `json:"price"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.jobRepo.AssignDevice(uint(jobID), deviceID, request.Price); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device assigned successfully"})
}

func (h *JobHandler) RemoveDeviceAPI(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
		return
	}

	deviceID := c.Param("deviceId")

	if err := h.jobRepo.RemoveDevice(uint(jobID), deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device removed successfully"})
}

func (h *JobHandler) BulkScanDevicesAPI(c *gin.Context) {
	var request models.BulkScanRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results, err := h.jobRepo.BulkAssignDevices(request.JobID, request.DeviceIDs, request.Price)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"results": results})
}