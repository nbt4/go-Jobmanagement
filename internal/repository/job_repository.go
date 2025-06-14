package repository

import (
	"fmt"
	"strings"
	"go-barcode-webapp/internal/models"

	"gorm.io/gorm"
)

type JobRepository struct {
	db *Database
}

func NewJobRepository(db *Database) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Create(job *models.Job) error {
	return r.db.Create(job).Error
}

func (r *JobRepository) GetByID(id uint) (*models.Job, error) {
	var job models.Job
	err := r.db.Preload("Customer").Preload("Status").Preload("JobDevices.Device.Product").First(&job, id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) Update(job *models.Job) error {
	return r.db.Save(job).Error
}

func (r *JobRepository) Delete(id uint) error {
	return r.db.Delete(&models.Job{}, id).Error
}

func (r *JobRepository) List(params *models.FilterParams) ([]models.JobWithDetails, error) {
	var jobs []models.JobWithDetails

	var sqlQuery string
	var args []interface{}

	sqlQuery = `SELECT j.jobID, j.customerID, j.statusID, 
			j.description, j.startDate, j.endDate, 
			j.revenue, j.final_revenue,
			CONCAT(COALESCE(c.companyname, ''), ' ', COALESCE(c.firstname, ''), ' ', COALESCE(c.lastname, '')) as customer_name, 
			s.status as status_name,
			COUNT(DISTINCT jd.deviceID) as device_count,
			COALESCE(j.final_revenue, j.revenue) as total_revenue
		FROM jobs j 
		LEFT JOIN customers c ON j.customerID = c.customerID
		LEFT JOIN status s ON j.statusID = s.statusID
		LEFT JOIN jobdevices jd ON j.jobID = jd.jobID`

	// Build WHERE conditions
	var conditions []string

	if params.StartDate != nil {
		conditions = append(conditions, "j.startDate >= ?")
		args = append(args, *params.StartDate)
	}
	if params.EndDate != nil {
		conditions = append(conditions, "j.endDate <= ?")
		args = append(args, *params.EndDate)
	}
	if params.CustomerID != nil {
		conditions = append(conditions, "j.customerID = ?")
		args = append(args, *params.CustomerID)
	}
	if params.StatusID != nil {
		conditions = append(conditions, "j.statusID = ?")
		args = append(args, *params.StatusID)
	}
	if params.MinRevenue != nil {
		conditions = append(conditions, "j.revenue >= ?")
		args = append(args, *params.MinRevenue)
	}
	if params.MaxRevenue != nil {
		conditions = append(conditions, "j.revenue <= ?")
		args = append(args, *params.MaxRevenue)
	}
	if params.SearchTerm != "" {
		searchPattern := "%" + params.SearchTerm + "%"
		conditions = append(conditions, "(j.description LIKE ? OR c.companyname LIKE ? OR c.firstname LIKE ? OR c.lastname LIKE ?)")
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	// Add WHERE clause if conditions exist
	if len(conditions) > 0 {
		sqlQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	sqlQuery += " GROUP BY j.jobID, j.customerID, j.statusID, j.description, j.startDate, j.endDate, j.revenue, j.final_revenue, customer_name, s.status"

	// Add ORDER BY
	sqlQuery += " ORDER BY j.jobID DESC"

	// Add pagination
	if params.Limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", params.Limit)
	}
	if params.Offset > 0 {
		sqlQuery += fmt.Sprintf(" OFFSET %d", params.Offset)
	}

	err := r.db.Raw(sqlQuery, args...).Scan(&jobs).Error
	return jobs, err
}

func (r *JobRepository) GetJobDevices(jobID uint) ([]models.JobDevice, error) {
	var jobDevices []models.JobDevice
	err := r.db.Where("jobID = ?", jobID).
		Preload("Device.Product").
		Find(&jobDevices).Error
	return jobDevices, err
}

func (r *JobRepository) AssignDevice(jobID uint, deviceID string, price float64) error {
	// Check if device is already assigned to another job
	var existingAssignment models.JobDevice
	err := r.db.Where("deviceID = ?", deviceID).First(&existingAssignment).Error
	if err == nil {
		return fmt.Errorf("device is already assigned to job %d", existingAssignment.JobID)
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Create new assignment
	jobDevice := &models.JobDevice{
		JobID:    jobID,
		DeviceID: deviceID,
	}

	// Only set custom price if it's greater than 0
	if price > 0 {
		jobDevice.CustomPrice = &price
	}

	err = r.db.Create(jobDevice).Error
	if err != nil {
		return err
	}

	// Recalculate and update job revenue
	return r.CalculateAndUpdateRevenue(jobID)
}

func (r *JobRepository) RemoveDevice(jobID uint, deviceID string) error {
	err := r.db.Where("jobID = ? AND deviceID = ?", jobID, deviceID).
		Delete(&models.JobDevice{}).Error
	if err != nil {
		return err
	}

	// Recalculate and update job revenue
	return r.CalculateAndUpdateRevenue(jobID)
}

func (r *JobRepository) BulkAssignDevices(jobID uint, deviceIDs []string, price float64) ([]models.ScanResult, error) {
	var results []models.ScanResult
	hasSuccessfulAssignments := false

	for _, deviceID := range deviceIDs {
		result := models.ScanResult{
			DeviceID: deviceID,
		}

		// Find device by serial number or device ID
		var device models.Device
		err := r.db.Where("serialnumber = ? OR deviceID = ?", deviceID, deviceID).First(&device).Error
		if err != nil {
			result.Success = false
			result.Message = "Device not found"
			results = append(results, result)
			continue
		}

		// Try to assign device (without triggering revenue calculation yet)
		err = r.assignDeviceWithoutRevenue(jobID, device.DeviceID, price)
		if err != nil {
			result.Success = false
			result.Message = err.Error()
		} else {
			result.Success = true
			result.Message = "Device assigned successfully"
			result.Device = &device
			hasSuccessfulAssignments = true
		}

		results = append(results, result)
	}

	// Calculate revenue once at the end for efficiency
	if hasSuccessfulAssignments {
		r.CalculateAndUpdateRevenue(jobID)
	}

	return results, nil
}

// Helper method to assign device without triggering revenue calculation
func (r *JobRepository) assignDeviceWithoutRevenue(jobID uint, deviceID string, price float64) error {
	// Check if device is already assigned to another job
	var existingAssignment models.JobDevice
	err := r.db.Where("deviceID = ?", deviceID).First(&existingAssignment).Error
	if err == nil {
		return fmt.Errorf("device is already assigned to job %d", existingAssignment.JobID)
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// Create new assignment
	jobDevice := &models.JobDevice{
		JobID:    jobID,
		DeviceID: deviceID,
	}

	// Only set custom price if it's greater than 0
	if price > 0 {
		jobDevice.CustomPrice = &price
	}

	return r.db.Create(jobDevice).Error
}

func (r *JobRepository) GetJobStats(jobID uint) (*models.JobWithDetails, error) {
	var job models.JobWithDetails
	err := r.db.Table("jobs j").
		Select(`j.*, c.name as customer_name, s.name as status_name,
				COUNT(DISTINCT jd.device_id) as device_count,
				COALESCE(SUM(jd.price), 0) as total_revenue`).
		Joins("LEFT JOIN customers c ON j.customer_id = c.id").
		Joins("LEFT JOIN statuses s ON j.status_id = s.id").
		Joins("LEFT JOIN job_devices jd ON j.id = jd.job_id AND jd.removed_at IS NULL").
		Where("j.id = ?", jobID).
		Group("j.id").
		First(&job).Error

	return &job, err
}

func (r *JobRepository) CalculateAndUpdateRevenue(jobID uint) error {
	// Get the job with dates
	var job models.Job
	err := r.db.First(&job, jobID).Error
	if err != nil {
		return err
	}

	// Calculate job duration in days
	var jobDays int = 1 // Default to 1 day if no dates set
	if job.StartDate != nil && job.EndDate != nil {
		duration := job.EndDate.Sub(*job.StartDate)
		jobDays = int(duration.Hours()/24) + 1 // +1 to include both start and end day
		if jobDays < 1 {
			jobDays = 1 // Minimum 1 day
		}
	}

	// Calculate total revenue from job devices
	var totalRevenue float64
	var jobDevices []models.JobDevice
	err = r.db.Where("jobID = ?", jobID).
		Preload("Device.Product").
		Find(&jobDevices).Error
	if err != nil {
		return err
	}

	for _, jd := range jobDevices {
		if jd.CustomPrice != nil && *jd.CustomPrice > 0 {
			// Use custom price as-is (flat rate, not per day)
			totalRevenue += *jd.CustomPrice
		} else if jd.Device.Product != nil && jd.Device.Product.ItemCostPerDay != nil {
			// Use daily rate × job duration
			totalRevenue += *jd.Device.Product.ItemCostPerDay * float64(jobDays)
		}
	}

	// Update the job revenue
	job.Revenue = totalRevenue
	
	// Calculate final revenue after discount
	var finalRevenue float64
	if job.DiscountType == "percent" {
		// Percentage discount
		finalRevenue = totalRevenue * (1 - job.Discount/100)
	} else {
		// Fixed amount discount
		finalRevenue = totalRevenue - job.Discount
		if finalRevenue < 0 {
			finalRevenue = 0 // Cannot be negative
		}
	}
	job.FinalRevenue = &finalRevenue
	
	return r.db.Save(&job).Error
}

func (r *JobRepository) UpdateFinalRevenue(jobID uint) error {
	// Get the job with current revenue
	var job models.Job
	err := r.db.First(&job, jobID).Error
	if err != nil {
		return err
	}

	// Calculate final revenue after discount using existing revenue
	var finalRevenue float64
	if job.DiscountType == "percent" {
		// Percentage discount
		finalRevenue = job.Revenue * (1 - job.Discount/100)
	} else {
		// Fixed amount discount
		finalRevenue = job.Revenue - job.Discount
		if finalRevenue < 0 {
			finalRevenue = 0 // Cannot be negative
		}
	}
	job.FinalRevenue = &finalRevenue
	
	return r.db.Save(&job).Error
}