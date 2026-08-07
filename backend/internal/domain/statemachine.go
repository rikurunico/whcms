package domain

import "github.com/tsdlamongan/whcms/backend/pkg/apperr"

// State machines (CONTRACTS.md §4):
//
//	invoice: draft->unpaid; unpaid->paid|cancelled|overdue; overdue->paid|cancelled; paid->refunded
//	order:   pending->active|fraud|cancelled
//	service: pending->active|cancelled; active->suspended|terminated; suspended->active|terminated
//	domain:  pending->active|cancelled; active->expired; pending_transfer->active|cancelled; expired->active(renew)

var invoiceTransitions = map[InvoiceStatus][]InvoiceStatus{
	InvoiceDraft:   {InvoiceUnpaid},
	InvoiceUnpaid:  {InvoicePaid, InvoiceCancelled, InvoiceOverdue},
	InvoiceOverdue: {InvoicePaid, InvoiceCancelled},
	InvoicePaid:    {InvoiceRefunded},
}

var orderTransitions = map[OrderStatus][]OrderStatus{
	OrderPending: {OrderActive, OrderFraud, OrderCancelled},
}

var serviceTransitions = map[ServiceStatus][]ServiceStatus{
	ServicePending:   {ServiceActive, ServiceCancelled},
	ServiceActive:    {ServiceSuspended, ServiceTerminated},
	ServiceSuspended: {ServiceActive, ServiceTerminated},
}

var domainTransitions = map[DomainStatus][]DomainStatus{
	DomainPending:         {DomainActive, DomainCancelled},
	DomainActive:          {DomainExpired},
	DomainPendingTransfer: {DomainActive, DomainCancelled},
	DomainExpired:         {DomainActive}, // renew
}

func canTransition[S comparable](table map[S][]S, from, to S) bool {
	for _, allowed := range table[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// InvoiceCanTransition reports whether an invoice may move from -> to.
func InvoiceCanTransition(from, to InvoiceStatus) bool {
	return canTransition(invoiceTransitions, from, to)
}

// OrderCanTransition reports whether an order may move from -> to.
func OrderCanTransition(from, to OrderStatus) bool {
	return canTransition(orderTransitions, from, to)
}

// ServiceCanTransition reports whether a service may move from -> to.
func ServiceCanTransition(from, to ServiceStatus) bool {
	return canTransition(serviceTransitions, from, to)
}

// DomainCanTransition reports whether a domain may move from -> to.
func DomainCanTransition(from, to DomainStatus) bool {
	return canTransition(domainTransitions, from, to)
}

// Typed transition funcs: validate the move and return the new status, or a
// CONFLICT apperr describing the illegal transition.

// TransitionInvoice validates and applies an invoice status transition.
func TransitionInvoice(from, to InvoiceStatus) (InvoiceStatus, error) {
	if !InvoiceCanTransition(from, to) {
		return from, apperr.Newf(apperr.CodeConflict, "invoice cannot transition from %s to %s", from, to)
	}
	return to, nil
}

// TransitionOrder validates and applies an order status transition.
func TransitionOrder(from, to OrderStatus) (OrderStatus, error) {
	if !OrderCanTransition(from, to) {
		return from, apperr.Newf(apperr.CodeConflict, "order cannot transition from %s to %s", from, to)
	}
	return to, nil
}

// TransitionService validates and applies a service status transition.
func TransitionService(from, to ServiceStatus) (ServiceStatus, error) {
	if !ServiceCanTransition(from, to) {
		return from, apperr.Newf(apperr.CodeConflict, "service cannot transition from %s to %s", from, to)
	}
	return to, nil
}

// TransitionDomain validates and applies a domain status transition.
func TransitionDomain(from, to DomainStatus) (DomainStatus, error) {
	if !DomainCanTransition(from, to) {
		return from, apperr.Newf(apperr.CodeConflict, "domain cannot transition from %s to %s", from, to)
	}
	return to, nil
}
