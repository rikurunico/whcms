package orders

import (
	"github.com/tsdlamongan/whcms/backend/internal/domain"
)

// CreateOrderRequest is the checkout payload (POST /orders). CaptchaToken is
// verified only when CAPTCHA is enabled (security.captcha_enabled).
type CreateOrderRequest struct {
	Items        []OrderItemRequest `json:"items" validate:"required,min=1,max=20,dive"`
	CouponCode   string             `json:"coupon_code" validate:"omitempty,min=1,max=64"`
	Notes        string             `json:"notes" validate:"omitempty,max=1000"`
	CaptchaToken string             `json:"captcha_token"`
}

// OrderItemRequest is one requested order line.
type OrderItemRequest struct {
	ItemType     string                   `json:"item_type" validate:"required,oneof=product domain_register domain_transfer"`
	ProductID    int64                    `json:"product_id" validate:"omitempty,gt=0"`
	Domain       string                   `json:"domain" validate:"omitempty,max=253"`
	Cycle        string                   `json:"cycle" validate:"omitempty,oneof=one_time monthly quarterly semiannually annually biennially"`
	Options      []OptionSelectionRequest `json:"options" validate:"omitempty,max=20,dive"`
	Specs        []SpecSelectionRequest   `json:"specs" validate:"omitempty,max=20,dive"`
	DomainYears  int                      `json:"domain_years" validate:"omitempty,min=1,max=10"`
	DomainAddons []string                 `json:"domain_addons" validate:"omitempty,max=3,dive,oneof=id_protection dns_management email_forwarding"`
	EPPCode      string                   `json:"epp_code" validate:"omitempty,max=255"`
}

// OptionSelectionRequest picks one configurable option value.
type OptionSelectionRequest struct {
	OptionID int64 `json:"option_id" validate:"required,gt=0"`
	ValueID  int64 `json:"value_id" validate:"required,gt=0"`
}

// SpecSelectionRequest is a customer-chosen quantity for one dynamic spec knob.
type SpecSelectionRequest struct {
	Key       string `json:"key" validate:"required,min=1,max=50"`
	Qty       int64  `json:"qty" validate:"gte=0"`
	Unlimited bool   `json:"unlimited"`
}

// OptionSelection is a resolved configurable option choice persisted in
// order_items.options.
type OptionSelection struct {
	OptionID int64  `json:"option_id"`
	ValueID  int64  `json:"value_id"`
	Value    string `json:"value"`
	Delta    int64  `json:"delta"`
}

// SpecSelection is a resolved, priced dynamic-spec choice persisted in
// order_items.options (and copied to services.panel_meta at activation). Qty is
// in the spec unit; domain.UnlimitedQty (-1) marks an unlimited choice.
type SpecSelection struct {
	Key          string `json:"key"`
	ProvisionKey string `json:"provision_key"`
	Unit         string `json:"unit"`
	Qty          int64  `json:"qty"`
	Unlimited    bool   `json:"unlimited,omitempty"`
	Amount       int64  `json:"amount"`
}

// itemOptions is the JSON object stored in order_items.options.
type itemOptions struct {
	Selections  []OptionSelection `json:"selections,omitempty"`
	Specs       []SpecSelection   `json:"specs,omitempty"`
	DomainYears int               `json:"domain_years,omitempty"`
	// DomainAddons are the addon keys (id_protection, dns_management,
	// email_forwarding) selected for a domain_register/domain_transfer item.
	DomainAddons []string `json:"domain_addons,omitempty"`
	// DomainRenewPerYear is the resolved base renewal rate (TLD/premium renew
	// price plus addon costs, excluding the initial term's register/transfer
	// price) frozen at checkout time - used to seed domains.recurring_amount
	// at activation instead of re-deriving it from the initial term's total.
	DomainRenewPerYear int64  `json:"domain_renew_per_year,omitempty"`
	EPPCodeEnc         string `json:"epp_code_enc,omitempty"`
}

// CheckoutResponse is returned by POST /orders: the created order (status
// pending, or fraud when a fraud rule fired), its items and the invoice
// (status unpaid, or cancelled for fraud orders).
type CheckoutResponse struct {
	Order   domain.Order       `json:"order"`
	Items   []domain.OrderItem `json:"items"`
	Invoice *domain.Invoice    `json:"invoice"`
}

// OrderDetail is one order with its items.
type OrderDetail struct {
	Order     domain.Order       `json:"order"`
	Items     []domain.OrderItem `json:"items"`
	InvoiceID int64              `json:"invoice_id,omitempty"`
}
