package domain_test

import (
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestProvisionKeyValid(t *testing.T) {
	valid := []domain.ProvisionKey{
		domain.SpecDisk, domain.SpecBandwidth, domain.SpecAddonDomains,
		domain.SpecSubdomains, domain.SpecParkedDomains, domain.SpecEmailAccounts,
		domain.SpecDatabases, domain.SpecFTPAccounts,
	}
	for _, k := range valid {
		assert.True(t, k.Valid(), string(k))
	}
	assert.False(t, domain.ProvisionKey("cpu").Valid())
	assert.False(t, domain.ProvisionKey("").Valid())
}

func TestSpecUnitValid(t *testing.T) {
	assert.True(t, domain.UnitGB.Valid())
	assert.True(t, domain.UnitMB.Valid())
	assert.True(t, domain.UnitCount.Valid())
	assert.False(t, domain.SpecUnit("tb").Valid())
}

func TestDynamicPackageName(t *testing.T) {
	limitsA := map[domain.ProvisionKey]int64{domain.SpecDisk: 20480, domain.SpecBandwidth: domain.UnlimitedQty}
	limitsB := map[domain.ProvisionKey]int64{domain.SpecBandwidth: domain.UnlimitedQty, domain.SpecDisk: 20480} // same content, built in a different order
	limitsDiffValue := map[domain.ProvisionKey]int64{domain.SpecDisk: 40960, domain.SpecBandwidth: domain.UnlimitedQty}
	limitsDiffKey := map[domain.ProvisionKey]int64{domain.SpecDisk: 20480, domain.SpecAddonDomains: 5}

	nameA := domain.DynamicPackageName("", limitsA, domain.PackageToggles{})
	nameB := domain.DynamicPackageName("", limitsB, domain.PackageToggles{})
	assert.Equal(t, nameA, nameB, "identical limits must resolve to the identical name regardless of map build order")

	assert.NotEqual(t, nameA, domain.DynamicPackageName("", limitsDiffValue, domain.PackageToggles{}), "a different limit value must change the name")
	assert.NotEqual(t, nameA, domain.DynamicPackageName("", limitsDiffKey, domain.PackageToggles{}), "a different limit key must change the name")
	assert.NotEqual(t, nameA, domain.DynamicPackageName("", limitsA, domain.PackageToggles{FeatureList: "custom_feature_list"}), "a different feature list must change the name")
	assert.NotEqual(t, nameA, domain.DynamicPackageName("", limitsA, domain.PackageToggles{ShellAccess: true}), "a different shell access toggle must change the name")
	assert.NotEqual(t, nameA, domain.DynamicPackageName("", limitsA, domain.PackageToggles{CGIAccess: true}), "a different cgi access toggle must change the name")
	assert.NotEqual(t, nameA, domain.DynamicPackageName("", limitsA, domain.PackageToggles{TemplatePackage: "custom_template"}), "a different template package must change the name")

	prefixed := domain.DynamicPackageName("reseller_", limitsA, domain.PackageToggles{})
	assert.Equal(t, "reseller_"+nameA, prefixed)

	assert.Regexp(t, `^[A-Za-z0-9_]{1,45}$`, nameA, "must satisfy cPanel's package-name rules")
	assert.Regexp(t, `^[A-Za-z0-9_]{1,45}$`, prefixed, "must satisfy cPanel's package-name rules even with a reseller prefix")

	assert.NotEmpty(t, domain.DynamicPackageName("", map[domain.ProvisionKey]int64{}, domain.PackageToggles{}), "an empty spec still yields a stable, valid name")
}

func TestToPanelMB(t *testing.T) {
	tests := []struct {
		name string
		qty  int64
		unit domain.SpecUnit
		want int64
	}{
		{"gb to mb", 10, domain.UnitGB, 10240},
		{"mb passthrough", 512, domain.UnitMB, 512},
		{"count passthrough", 5, domain.UnitCount, 5},
		{"unlimited gb", domain.UnlimitedQty, domain.UnitGB, domain.UnlimitedQty},
		{"unlimited count", domain.UnlimitedQty, domain.UnitCount, domain.UnlimitedQty},
		{"zero gb", 0, domain.UnitGB, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.ToPanelMB(tt.qty, tt.unit))
		})
	}
}

func TestPanelParam(t *testing.T) {
	tests := []struct {
		module   domain.ServerModuleName
		key      domain.ProvisionKey
		wantName string
		wantSize bool
		wantOK   bool
	}{
		{domain.ModuleCpanel, domain.SpecDisk, "quota", true, true},
		{domain.ModuleCpanel, domain.SpecBandwidth, "bwlimit", true, true},
		{domain.ModuleCpanel, domain.SpecAddonDomains, "maxaddon", false, true},
		{domain.ModuleCpanel, domain.SpecSubdomains, "maxsub", false, true},
		{domain.ModuleCpanel, domain.SpecParkedDomains, "maxpark", false, true},
		{domain.ModuleCpanel, domain.SpecEmailAccounts, "maxpop", false, true},
		{domain.ModuleCpanel, domain.SpecDatabases, "maxsql", false, true},
		{domain.ModuleCpanel, domain.SpecFTPAccounts, "maxftp", false, true},
		{domain.ModuleDirectAdmin, domain.SpecDisk, "quota", true, true},
		{domain.ModuleDirectAdmin, domain.SpecBandwidth, "bandwidth", true, true},
		{domain.ModuleDirectAdmin, domain.SpecAddonDomains, "vdomains", false, true},
		{domain.ModuleDirectAdmin, domain.SpecSubdomains, "nsubdomains", false, true},
		{domain.ModuleDirectAdmin, domain.SpecParkedDomains, "domainptr", false, true},
		{domain.ModuleDirectAdmin, domain.SpecEmailAccounts, "nemails", false, true},
		{domain.ModuleDirectAdmin, domain.SpecDatabases, "mysql", false, true},
		{domain.ModuleDirectAdmin, domain.SpecFTPAccounts, "ftp", false, true},
		{domain.ModuleNone, domain.SpecDisk, "", false, false},
		{domain.ModuleCpanel, domain.ProvisionKey("cpu"), "", false, false},
	}
	for _, tt := range tests {
		name, isSize, ok := domain.PanelParam(tt.module, tt.key)
		assert.Equal(t, tt.wantName, name, "%s/%s name", tt.module, tt.key)
		assert.Equal(t, tt.wantSize, isSize, "%s/%s isSize", tt.module, tt.key)
		assert.Equal(t, tt.wantOK, ok, "%s/%s ok", tt.module, tt.key)
	}
}
