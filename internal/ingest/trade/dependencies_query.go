package trade

import (
	"sort"
	"strings"
)

// BuildDependencyResolved returns supplier breakdown, preferring processed
// trade_dependencies.json when available.
func BuildDependencyResolved(resolved ResolvedTrade, importer, commodity string) Dependency {
	if resolved.DependencyFile != nil {
		return BuildDependencyFromDependencies(*resolved.DependencyFile, importer, commodity)
	}
	return BuildDependency(resolved.File, importer, commodity)
}

// BuildConcentrationResolved returns HHI concentration from resolved trade data.
func BuildConcentrationResolved(resolved ResolvedTrade, importer, commodity string) Concentration {
	return concentrationFromDependency(BuildDependencyResolved(resolved, importer, commodity))
}

// BuildDependencyFromDependencies filters processed dependency rows by importer
// and commodity, returning every matching exporter with stored or recalculated shares.
func BuildDependencyFromDependencies(df DependencyFile, importer, commodity string) Dependency {
	importer = strings.TrimSpace(importer)
	commodity = strings.TrimSpace(commodity)

	dep := Dependency{ImporterCode: strings.ToUpper(importer), Commodity: commodity}
	for _, d := range df.Dependencies {
		if !matchDependencyImporter(d, importer) || !matchDependencyCommodity(d, commodity) {
			continue
		}
		dep.HasData = true
		dep.ImporterName = d.Importer
		dep.Commodity = d.Commodity
		dep.TotalImportsUSD += d.TradeValueUSD
		dep.Suppliers = append(dep.Suppliers, Supplier{
			ExporterName: d.Exporter,
			ValueUSD:     d.TradeValueUSD,
			Share:        d.Share,
			Dependency:   SupplierDependencyBand(d.Share),
		})
	}
	if !dep.HasData {
		return dep
	}

	if dep.TotalImportsUSD > 0 {
		for i := range dep.Suppliers {
			if dep.Suppliers[i].Share <= 0 {
				dep.Suppliers[i].Share = dep.Suppliers[i].ValueUSD / dep.TotalImportsUSD
				dep.Suppliers[i].Dependency = SupplierDependencyBand(dep.Suppliers[i].Share)
			}
		}
	}

	sort.SliceStable(dep.Suppliers, func(i, j int) bool {
		if dep.Suppliers[i].Share != dep.Suppliers[j].Share {
			return dep.Suppliers[i].Share > dep.Suppliers[j].Share
		}
		if dep.Suppliers[i].ValueUSD != dep.Suppliers[j].ValueUSD {
			return dep.Suppliers[i].ValueUSD > dep.Suppliers[j].ValueUSD
		}
		return dep.Suppliers[i].ExporterName < dep.Suppliers[j].ExporterName
	})
	return dep
}

func matchDependencyImporter(d TradeDependency, q string) bool {
	if strings.EqualFold(d.Importer, q) {
		return true
	}
	canonQ := NormalizeImporterQuery(q)
	canonD := NormalizeImporterQuery(d.Importer)
	if canonQ != "" && strings.EqualFold(canonQ, canonD) {
		return true
	}
	return strings.EqualFold(NormalizeCountryName(q), NormalizeCountryName(d.Importer))
}

func matchDependencyCommodity(d TradeDependency, q string) bool {
	return strings.EqualFold(d.Commodity, q) || strings.EqualFold(d.HSCode, q)
}
