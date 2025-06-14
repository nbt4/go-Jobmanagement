package repository

import (
	"go-barcode-webapp/internal/models"
)

type CustomerRepository struct {
	db *Database
}

func NewCustomerRepository(db *Database) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) Create(customer *models.Customer) error {
	return r.db.Create(customer).Error
}

func (r *CustomerRepository) GetByID(id uint) (*models.Customer, error) {
	var customer models.Customer
	err := r.db.Preload("Jobs").First(&customer, id).Error
	if err != nil {
		return nil, err
	}
	return &customer, nil
}

func (r *CustomerRepository) Update(customer *models.Customer) error {
	return r.db.Save(customer).Error
}

func (r *CustomerRepository) Delete(id uint) error {
	return r.db.Delete(&models.Customer{}, id).Error
}

func (r *CustomerRepository) List(params *models.FilterParams) ([]models.Customer, error) {
	var customers []models.Customer

	query := r.db.Model(&models.Customer{})

	if params.SearchTerm != "" {
		searchPattern := "%" + params.SearchTerm + "%"
		query = query.Where("companyname LIKE ? OR firstname LIKE ? OR lastname LIKE ? OR email LIKE ?", searchPattern, searchPattern, searchPattern, searchPattern)
	}

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	query = query.Order("companyname ASC")

	err := query.Find(&customers).Error
	return customers, err
}