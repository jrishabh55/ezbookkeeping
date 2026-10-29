package models

import "github.com/mayswind/ezbookkeeping/pkg/utils"

// Product is an item of the user's catalog; RenewalMonths 0 means a one-off product, which never carries a renewal mode
type Product struct {
	ProductId          int64  `xorm:"PK"`
	Uid                int64  `xorm:"INDEX(IDX_product_uid_deleted) NOT NULL"`
	Deleted            bool   `xorm:"INDEX(IDX_product_uid_deleted) NOT NULL"`
	Name               string `xorm:"VARCHAR(128) NOT NULL"`
	Description        string `xorm:"VARCHAR(512) NOT NULL"`
	Sac                string `xorm:"VARCHAR(8) NOT NULL DEFAULT ''"`
	DefaultRate        int64  `xorm:"NOT NULL"`
	Currency           string `xorm:"VARCHAR(3) NOT NULL DEFAULT ''"` // empty means the profile's default currency
	RenewalMonths      int32  `xorm:"NOT NULL DEFAULT 0"`
	DefaultRenewalMode string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	Archived           bool   `xorm:"NOT NULL DEFAULT false"`
	CreatedUnixTime    int64
	UpdatedUnixTime    int64
	DeletedUnixTime    int64
}

// ProductCreateRequest represents all parameters of product creation request
type ProductCreateRequest struct {
	Name               string `json:"name" binding:"required,notBlank,max=128"`
	Description        string `json:"description" binding:"max=512"`
	Sac                string `json:"sac" binding:"max=8"`
	DefaultRate        int64  `json:"defaultRate" binding:"min=0,max=10000000000"`
	Currency           string `json:"currency" binding:"max=3"`
	RenewalMonths      int32  `json:"renewalMonths" binding:"min=0,max=120"`
	DefaultRenewalMode string `json:"defaultRenewalMode" binding:"max=16"`
	Archived           bool   `json:"archived"`
}

// ProductModifyRequest represents all parameters of product modification request
type ProductModifyRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
	ProductCreateRequest
}

// ProductResponse represents a view-object of product
type ProductResponse struct {
	Id                 string `json:"id"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	Sac                string `json:"sac"`
	DefaultRate        int64  `json:"defaultRate"`
	Currency           string `json:"currency"`
	RenewalMonths      int32  `json:"renewalMonths"`
	DefaultRenewalMode string `json:"defaultRenewalMode"`
	Archived           bool   `json:"archived"`
}

// ProductLinkLinesRequest links every unlinked invoice line whose item name matches (trimmed, case-insensitive) to a product
type ProductLinkLinesRequest struct {
	ProductId int64  `json:"productId,string" binding:"required,min=1"`
	ItemName  string `json:"itemName" binding:"required,notBlank,max=128"`
}

// InvoiceLineGroupResponse is a group of unlinked invoice lines sharing an item name
type InvoiceLineGroupResponse struct {
	ItemName string `json:"itemName"`
	Count    int    `json:"count"`
}

// ToProductResponse returns a view-object according to database model
func (p *Product) ToProductResponse() *ProductResponse {
	return &ProductResponse{
		Id:                 utils.Int64ToString(p.ProductId),
		Name:               p.Name,
		Description:        p.Description,
		Sac:                p.Sac,
		DefaultRate:        p.DefaultRate,
		Currency:           p.Currency,
		RenewalMonths:      p.RenewalMonths,
		DefaultRenewalMode: p.DefaultRenewalMode,
		Archived:           p.Archived,
	}
}
