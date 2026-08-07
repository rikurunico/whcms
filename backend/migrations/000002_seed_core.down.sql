DELETE FROM email_templates WHERE key IN (
    'verify_email','reset_password','invoice_created','payment_received',
    'service_activated','service_suspended','service_unsuspended','service_terminated',
    'invoice_reminder','invoice_overdue','domain_registered','domain_renewed',
    'ticket_opened','ticket_replied','admin_alert'
);

DELETE FROM registrars WHERE name = 'rdash';

DELETE FROM ticket_departments WHERE name IN ('Support','Billing');

DELETE FROM settings WHERE key IN (
    'company.name','company.logo_key','company.address','company.email',
    'billing.tax_enabled','billing.tax_rate','billing.tax_inclusive',
    'billing.invoice_due_days','billing.renewal_lead_days',
    'billing.late_fee_enabled','billing.late_fee_amount',
    'billing.reminder_days','billing.overdue_reminder_days',
    'automation.suspend_after_days','automation.terminate_after_days',
    'mail.from_name','mail.from_email',
    'tickets.allowed_extensions','tickets.max_attachment_mb'
);
