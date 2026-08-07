package domain

import (
	"fmt"
	"hash/fnv"
	"sort"
	"time"
)

// ProvisionKey is a canonical, panel-agnostic hosting resource knob. Each key
// maps to a concrete cPanel/DirectAdmin parameter via PanelParam.
type ProvisionKey string

const (
	SpecDisk          ProvisionKey = "disk"
	SpecBandwidth     ProvisionKey = "bandwidth"
	SpecAddonDomains  ProvisionKey = "addon_domains"
	SpecSubdomains    ProvisionKey = "subdomains"
	SpecParkedDomains ProvisionKey = "parked_domains"
	SpecEmailAccounts ProvisionKey = "email_accounts"
	SpecDatabases     ProvisionKey = "databases"
	SpecFTPAccounts   ProvisionKey = "ftp_accounts"
)

// Valid reports whether the key is a known provision key.
func (k ProvisionKey) Valid() bool {
	switch k {
	case SpecDisk, SpecBandwidth, SpecAddonDomains, SpecSubdomains,
		SpecParkedDomains, SpecEmailAccounts, SpecDatabases, SpecFTPAccounts:
		return true
	}
	return false
}

// SpecUnit is the unit a spec quantity is expressed in.
type SpecUnit string

const (
	UnitGB    SpecUnit = "gb"
	UnitMB    SpecUnit = "mb"
	UnitCount SpecUnit = "count"
)

// Valid reports whether the unit is a known value.
func (u SpecUnit) Valid() bool {
	switch u {
	case UnitGB, UnitMB, UnitCount:
		return true
	}
	return false
}

// UnlimitedQty is the sentinel quantity meaning "no cap". It is carried through
// pricing and provisioning and rendered per-panel by each adapter.
const UnlimitedQty int64 = -1

// services.panel_meta keys shared between order activation (writer) and
// provisioning (reader) for dynamic/custom-spec products.
const (
	PanelMetaChosenSpecs = "chosen_specs" // customer-chosen spec breakdown snapshot
	PanelMetaPackageName = "package_name" // the dynamic package name in use by this service (may be shared with siblings resolving to identical limits)
	PanelMetaLimits      = "limits"       // resolved panel limits pushed to the package
)

// ProductSpec is one configurable knob on a product (e.g. disk, bandwidth).
type ProductSpec struct {
	ID             int64        `json:"id"`
	ProductID      int64        `json:"product_id"`
	Key            string       `json:"key"`
	Label          string       `json:"label"`
	ProvisionKey   ProvisionKey `json:"provision_key"`
	Unit           SpecUnit     `json:"unit"`
	IncludedQty    int64        `json:"included_qty"`
	MinQty         int64        `json:"min_qty"`
	MaxQty         int64        `json:"max_qty"` // 0 = unbounded
	StepQty        int64        `json:"step_qty"`
	DefaultQty     int64        `json:"default_qty"`
	AllowUnlimited bool         `json:"allow_unlimited"`
	Sort           int          `json:"sort"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// ProductSpecPricing is the per-cycle IDR unit pricing of a spec.
type ProductSpecPricing struct {
	ID             int64        `json:"id"`
	SpecID         int64        `json:"spec_id"`
	Cycle          BillingCycle `json:"cycle"`
	UnitPrice      int64        `json:"unit_price"`      // per spec-unit above IncludedQty
	UnlimitedPrice int64        `json:"unlimited_price"` // flat add-on when unlimited chosen
	Currency       string       `json:"currency"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// PackageToggles bundles the package-identity-affecting flags that ride
// alongside resolved limits in a dynamic package's name hash. Every field
// here changes what EnsurePackage sends to the panel, so it MUST be
// included here too - otherwise two services with identical Limits but
// different toggles would incorrectly collide onto the same physical
// package.
type PackageToggles struct {
	FeatureList     string // cPanel only; "" defaults to WHM's "default"
	ShellAccess     bool   // cPanel hasshell / DA ssh
	CGIAccess       bool   // cPanel cgi / DA cgi
	TemplatePackage string // DirectAdmin only
}

// DynamicPackageName is the deterministic control-panel package name for a
// set of dynamic/custom-spec resource limits. It depends only on the resolved
// limits and package toggles - never the service or product - so any two
// services that resolve to identical limits and toggles on the same server
// land on the same name: EnsurePackage's create-then-update-on-conflict
// semantics then naturally reuse the existing package instead of creating a
// near-duplicate per service. prefix is the owning server's PackagePrefix -
// required by some real WHM reseller accounts (blank on a dedicated/non-
// reseller server, the common case).
func DynamicPackageName(prefix string, limits map[ProvisionKey]int64, toggles PackageToggles) string {
	keys := make([]string, 0, len(limits))
	for k := range limits {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)

	h := fnv.New64a()
	for _, k := range keys {
		fmt.Fprintf(h, "%s=%d;", k, limits[ProvisionKey(k)])
	}
	fmt.Fprintf(h, "feat=%s;shell=%t;cgi=%t;tmpl=%s",
		toggles.FeatureList, toggles.ShellAccess, toggles.CGIAccess, toggles.TemplatePackage)

	return fmt.Sprintf("%swhcms_spec_%x", prefix, h.Sum64())
}

// ToPanelMB converts a quantity in its spec unit to MB for size-type panel
// parameters. Non-size units (count) are returned unchanged. The unlimited
// sentinel is passed through untouched.
func ToPanelMB(qty int64, u SpecUnit) int64 {
	if qty == UnlimitedQty {
		return UnlimitedQty
	}
	switch u {
	case UnitGB:
		return qty * 1024
	default: // mb, count
		return qty
	}
}

// PanelParam maps a canonical knob to the concrete WHM/DirectAdmin parameter
// name for the given module. isSize reports whether the value is a size (MB)
// rather than a count. ok is false for unknown (module, key) combinations.
func PanelParam(module ServerModuleName, key ProvisionKey) (name string, isSize bool, ok bool) {
	switch module {
	case ModuleCpanel:
		switch key {
		case SpecDisk:
			return "quota", true, true
		case SpecBandwidth:
			return "bwlimit", true, true
		case SpecAddonDomains:
			return "maxaddon", false, true
		case SpecSubdomains:
			return "maxsub", false, true
		case SpecParkedDomains:
			return "maxpark", false, true
		case SpecEmailAccounts:
			return "maxpop", false, true
		case SpecDatabases:
			return "maxsql", false, true
		case SpecFTPAccounts:
			return "maxftp", false, true
		}
	case ModuleDirectAdmin:
		switch key {
		case SpecDisk:
			return "quota", true, true
		case SpecBandwidth:
			return "bandwidth", true, true
		case SpecAddonDomains:
			return "vdomains", false, true
		case SpecSubdomains:
			return "nsubdomains", false, true
		case SpecParkedDomains:
			return "domainptr", false, true
		case SpecEmailAccounts:
			return "nemails", false, true
		case SpecDatabases:
			return "mysql", false, true
		case SpecFTPAccounts:
			return "ftp", false, true
		}
	}
	return "", false, false
}
