// Command generate_strategic_global writes data/strategic_global/*.json.
// Run from repo root: go run ./tools/generate_strategic_global
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type entity struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type entitiesFile struct {
	Countries   []entity `json:"countries"`
	Commodities []entity `json:"commodities"`
	Sectors     []entity `json:"sectors"`
	Routes      []entity `json:"routes"`
	Companies   []entity `json:"companies"`
}

type dep struct {
	Source        string  `json:"source"`
	Target        string  `json:"target"`
	Relationship  string  `json:"relationship_type"`
	Weight        float64 `json:"weight"`
	Concentration float64 `json:"concentration"`
	Commodity     string  `json:"commodity,omitempty"`
	Sector        string  `json:"sector,omitempty"`
	Description   string  `json:"description,omitempty"`
}

type scenario struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Source       string  `json:"source"`
	Commodity    string  `json:"commodity"`
	ShockType    string  `json:"shock_type"`
	ShockPercent float64 `json:"shock_percent"`
	Depth        int     `json:"depth"`
	Description  string  `json:"description"`
}

func main() {
	outDir := filepath.Join("data", "strategic_global")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	ents := buildEntities()
	deps := buildDependencies()
	scens := buildScenarios()

	if err := writeJSON(filepath.Join(outDir, "entities.json"), ents); err != nil {
		fatal(err)
	}
	if err := writeJSON(filepath.Join(outDir, "dependencies.json"), map[string]any{"dependencies": deps}); err != nil {
		fatal(err)
	}
	if err := writeJSON(filepath.Join(outDir, "scenarios.json"), map[string]any{"scenarios": scens}); err != nil {
		fatal(err)
	}

	fmt.Printf("Wrote %s (%d countries, %d commodities, %d sectors, %d routes, %d deps, %d scenarios)\n",
		outDir, len(ents.Countries), len(ents.Commodities), len(ents.Sectors), len(ents.Routes),
		len(deps), len(scens))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func buildEntities() entitiesFile {
	countries := []entity{
		{Name: "United States", Description: "Largest economy; chip design, energy production, and defense manufacturing."},
		{Name: "China", Description: "Manufacturing hub, rare-earth processor, and major commodity importer."},
		{Name: "Taiwan", Description: "Dominant advanced semiconductor fabrication capacity."},
		{Name: "Japan", Description: "Precision manufacturing, automotive electronics, and industrial machinery."},
		{Name: "Korea, Rep.", Description: "Memory semiconductors, batteries, and shipbuilding."},
		{Name: "Germany", Description: "Industrial machinery, automotive, and chemicals."},
		{Name: "India", Description: "Growing electronics assembly, pharmaceuticals, and agriculture."},
		{Name: "Saudi Arabia", Description: "Swing crude oil producer and regional energy exporter."},
		{Name: "Democratic Republic of the Congo", Description: "Dominant mined cobalt supplier."},
		{Name: "Russia", Description: "Major natural gas, wheat, and fertilizer exporter."},
		{Name: "Ukraine", Description: "Key wheat, corn, and fertilizer transit economy."},
		{Name: "Brazil", Description: "Agricultural exporter and iron ore producer."},
		{Name: "Mexico", Description: "Electronics manufacturing and automotive supply chains."},
		{Name: "Canada", Description: "Energy, uranium, and industrial metals producer."},
		{Name: "Vietnam", Description: "Electronics assembly and textile manufacturing."},
		{Name: "Netherlands", Description: "European logistics hub and energy import gateway."},
		{Name: "Singapore", Description: "Maritime chokepoint economy and refining hub."},
		{Name: "United Arab Emirates", Description: "Gulf energy exporter and logistics node."},
		{Name: "Australia", Description: "Lithium, iron ore, and LNG exporter."},
		{Name: "Indonesia", Description: "Nickel processing and palm-oil economy."},
		{Name: "South Africa", Description: "Platinum group metals and industrial minerals."},
		{Name: "Chile", Description: "Largest lithium brine producer."},
		{Name: "Peru", Description: "Copper mining and agricultural exports."},
		{Name: "Argentina", Description: "Lithium triangle producer and grain exporter."},
	}

	commodities := []entity{
		{Name: "semiconductors", Description: "Advanced logic, memory, and packaging."},
		{Name: "crude oil", Description: "Unrefined petroleum feedstock."},
		{Name: "natural gas", Description: "Pipeline and regional gas supply."},
		{Name: "LNG", Description: "Liquefied natural gas for maritime trade."},
		{Name: "lithium", Description: "Battery-grade lithium compounds."},
		{Name: "cobalt", Description: "Battery cathode and aerospace alloys."},
		{Name: "nickel", Description: "Stainless steel and battery cathodes."},
		{Name: "copper", Description: "Electrical wiring and grid infrastructure."},
		{Name: "rare earths", Description: "Magnets, optics, and defense inputs."},
		{Name: "wheat", Description: "Staple grain for food security."},
		{Name: "corn", Description: "Feed grain and ethanol feedstock."},
		{Name: "rice", Description: "Staple grain for Asia and Africa."},
		{Name: "fertilizer", Description: "Nitrogen, phosphate, and potash inputs."},
		{Name: "uranium", Description: "Nuclear fuel feedstock."},
		{Name: "steel", Description: "Construction and industrial feedstock."},
		{Name: "aluminum", Description: "Lightweight metals for transport and packaging."},
		{Name: "batteries", Description: "Finished lithium-ion battery cells and packs."},
		{Name: "solar panels", Description: "Photovoltaic modules and polysilicon chain."},
		{Name: "pharmaceuticals", Description: "Active ingredients and finished medicines."},
		{Name: "shipping containers", Description: "Intermodal freight capacity proxy."},
	}

	sectors := []entity{
		{Name: "AI hardware", Description: "GPUs, accelerators, and AI compute."},
		{Name: "cloud infrastructure", Description: "Hyperscale data centres."},
		{Name: "automotive electronics", Description: "Vehicle ECUs and sensors."},
		{Name: "consumer devices", Description: "Phones, PCs, and wearables."},
		{Name: "electronics manufacturing", Description: "Contract assembly and EMS."},
		{Name: "EV batteries", Description: "Electric vehicle battery packs."},
		{Name: "renewable energy", Description: "Wind, solar, and grid storage."},
		{Name: "power generation", Description: "Utilities and baseload power."},
		{Name: "agriculture", Description: "Farming and agri-inputs."},
		{Name: "food security", Description: "National grain reserves and food systems."},
		{Name: "defense manufacturing", Description: "Military systems and munitions."},
		{Name: "aerospace manufacturing", Description: "Airframes, engines, and avionics."},
		{Name: "medical supply chains", Description: "Hospitals and pharma distribution."},
		{Name: "construction", Description: "Buildings and civil infrastructure."},
		{Name: "shipping logistics", Description: "Freight, ports, and maritime trade."},
		{Name: "energy-intensive manufacturing", Description: "Steel, chemicals, and smelting."},
		{Name: "data centers", Description: "Colocation and enterprise compute."},
		{Name: "industrial machinery", Description: "Machine tools and capital goods."},
		{Name: "telecommunications", Description: "5G networks and backbone fibre."},
		{Name: "semiconductor fabrication", Description: "Foundry and packaging capacity."},
	}

	routes := []entity{
		{Name: "Suez Canal", Description: "Asia–Europe maritime shortcut."},
		{Name: "Panama Canal", Description: "Atlantic–Pacific container corridor."},
		{Name: "Strait of Hormuz", Description: "Gulf crude and LNG transit chokepoint."},
		{Name: "Red Sea", Description: "Suez approach and Bab el-Mandeb corridor."},
		{Name: "Taiwan Strait", Description: "Semiconductor and electronics shipping lane."},
		{Name: "South China Sea", Description: "East Asian manufacturing trade corridor."},
		{Name: "Black Sea", Description: "Grain and fertilizer export corridor."},
		{Name: "Malacca Strait", Description: "Indian Ocean–Pacific energy and goods lane."},
	}

	return entitiesFile{Countries: countries, Commodities: commodities, Sectors: sectors, Routes: routes, Companies: []entity{}}
}

func d(source, target, rel, commodity, sector, desc string, weight, conc float64) dep {
	return dep{
		Source: source, Target: target, Relationship: rel,
		Weight: weight, Concentration: conc,
		Commodity: commodity, Sector: sector, Description: desc,
	}
}

func buildDependencies() []dep {
	var out []dep
	add := func(items ...dep) { out = append(out, items...) }

	// --- semiconductors chain ------------------------------------------------
	add(
		d("Taiwan", "semiconductors", "exports", "semiconductors", "", "Taiwan dominates advanced fab output.", 0.95, 0.92),
		d("Korea, Rep.", "semiconductors", "exports", "semiconductors", "", "Korea is a major memory producer.", 0.72, 0.58),
		d("United States", "semiconductors", "exports", "semiconductors", "", "US design-linked fabrication.", 0.55, 0.38),
		d("Japan", "semiconductors", "exports", "semiconductors", "", "Japanese specialty semiconductors.", 0.48, 0.35),
		d("China", "semiconductors", "exports", "semiconductors", "", "Chinese mature-node production.", 0.42, 0.28),
		d("semiconductors", "United States", "imports", "semiconductors", "", "US compute imports advanced chips.", 0.88, 0.82),
		d("semiconductors", "China", "imports", "semiconductors", "", "China assembly imports leading-edge chips.", 0.78, 0.70),
		d("semiconductors", "Japan", "imports", "semiconductors", "", "Japanese industry chip imports.", 0.70, 0.62),
		d("semiconductors", "Germany", "imports", "semiconductors", "", "German automotive chip demand.", 0.65, 0.55),
		d("semiconductors", "Korea, Rep.", "imports", "semiconductors", "", "Korean device makers import logic.", 0.58, 0.48),
		d("semiconductors", "India", "imports", "semiconductors", "", "India electronics import growth.", 0.52, 0.42),
		d("semiconductors", "Vietnam", "imports", "semiconductors", "", "Vietnam EMS chip imports.", 0.55, 0.45),
		d("Taiwan", "semiconductor fabrication", "industry_dependency", "semiconductors", "semiconductor fabrication", "Taiwan fab cluster.", 0.92, 0.88),
		d("Korea, Rep.", "semiconductor fabrication", "industry_dependency", "semiconductors", "semiconductor fabrication", "Korean memory fabs.", 0.80, 0.72),
		d("United States", "AI hardware", "industry_dependency", "semiconductors", "AI hardware", "US AI accelerator ecosystem.", 0.85, 0.75),
		d("United States", "cloud infrastructure", "industry_dependency", "semiconductors", "cloud infrastructure", "US hyperscale build-out.", 0.82, 0.70),
		d("China", "electronics manufacturing", "industry_dependency", "semiconductors", "electronics manufacturing", "Chinese EMS sector.", 0.80, 0.68),
		d("Vietnam", "electronics manufacturing", "industry_dependency", "semiconductors", "electronics manufacturing", "Vietnam assembly hubs.", 0.68, 0.55),
		d("Mexico", "automotive electronics", "industry_dependency", "semiconductors", "automotive electronics", "Nearshored auto electronics.", 0.62, 0.50),
		d("Japan", "automotive electronics", "industry_dependency", "semiconductors", "automotive electronics", "Japanese auto electronics.", 0.75, 0.62),
		d("Germany", "automotive electronics", "industry_dependency", "semiconductors", "automotive electronics", "German automotive supply chain.", 0.72, 0.58),
		d("Korea, Rep.", "consumer devices", "industry_dependency", "semiconductors", "consumer devices", "Korean consumer electronics.", 0.70, 0.58),
		d("China", "consumer devices", "industry_dependency", "semiconductors", "consumer devices", "Chinese device assembly.", 0.78, 0.65),
		d("AI hardware", "cloud infrastructure", "used_by", "semiconductors", "cloud infrastructure", "Cloud depends on AI accelerators.", 0.82, 0.68),
		d("AI hardware", "data centers", "used_by", "semiconductors", "data centers", "Data centres deploy AI hardware.", 0.78, 0.65),
		d("semiconductor fabrication", "telecommunications", "used_by", "semiconductors", "telecommunications", "5G chips from fabs.", 0.65, 0.52),
		d("semiconductors", "defense manufacturing", "used_by", "semiconductors", "defense manufacturing", "Defense systems need secure chips.", 0.70, 0.58),
	)

	// --- energy: crude, gas, LNG ---------------------------------------------
	add(
		d("Saudi Arabia", "crude oil", "exports", "crude oil", "", "Saudi swing crude production.", 0.92, 0.88),
		d("Russia", "crude oil", "exports", "crude oil", "", "Russian crude exports.", 0.78, 0.65),
		d("United Arab Emirates", "crude oil", "exports", "crude oil", "", "UAE Gulf crude exports.", 0.68, 0.58),
		d("United States", "crude oil", "exports", "crude oil", "", "US shale crude production.", 0.62, 0.48),
		d("Canada", "crude oil", "exports", "crude oil", "", "Canadian oil sands exports.", 0.55, 0.42),
		d("crude oil", "China", "imports", "crude oil", "", "China crude import demand.", 0.82, 0.75),
		d("crude oil", "India", "imports", "crude oil", "", "India growing oil imports.", 0.75, 0.68),
		d("crude oil", "Japan", "imports", "crude oil", "", "Japan energy import reliance.", 0.80, 0.72),
		d("crude oil", "Germany", "imports", "crude oil", "", "German industrial oil demand.", 0.68, 0.58),
		d("crude oil", "United States", "imports", "crude oil", "", "US refinery crude imports.", 0.45, 0.35),
		d("crude oil", "Netherlands", "imports", "crude oil", "", "Rotterdam refining hub.", 0.58, 0.48),
		d("Russia", "natural gas", "exports", "natural gas", "", "Russian pipeline gas.", 0.88, 0.82),
		d("United States", "natural gas", "exports", "natural gas", "", "US shale gas production.", 0.75, 0.62),
		d("Canada", "natural gas", "exports", "natural gas", "", "Canadian gas to US.", 0.55, 0.45),
		d("natural gas", "Germany", "imports", "natural gas", "", "German gas import dependence.", 0.78, 0.70),
		d("natural gas", "Japan", "imports", "natural gas", "", "Japan LNG/gas imports.", 0.72, 0.65),
		d("natural gas", "China", "imports", "natural gas", "", "Chinese gas demand growth.", 0.65, 0.55),
		d("Australia", "LNG", "exports", "LNG", "", "Australian LNG exports.", 0.82, 0.72),
		d("United Arab Emirates", "LNG", "exports", "LNG", "", "UAE LNG export capacity.", 0.58, 0.48),
		d("United States", "LNG", "exports", "LNG", "", "US Gulf Coast LNG.", 0.72, 0.60),
		d("LNG", "Japan", "imports", "LNG", "", "Japan LNG import reliance.", 0.85, 0.78),
		d("LNG", "China", "imports", "LNG", "", "Chinese LNG imports.", 0.70, 0.62),
		d("LNG", "Korea, Rep.", "imports", "LNG", "", "Korean LNG demand.", 0.68, 0.58),
		d("LNG", "India", "imports", "LNG", "", "Indian LNG growth.", 0.55, 0.45),
		d("crude oil", "shipping logistics", "used_by", "crude oil", "shipping logistics", "Maritime fuel from crude.", 0.75, 0.62),
		d("crude oil", "energy-intensive manufacturing", "used_by", "crude oil", "energy-intensive manufacturing", "Industrial fuel demand.", 0.72, 0.58),
		d("natural gas", "power generation", "used_by", "natural gas", "power generation", "Gas-fired power plants.", 0.78, 0.65),
		d("natural gas", "energy-intensive manufacturing", "used_by", "natural gas", "energy-intensive manufacturing", "Gas for chemicals and heat.", 0.68, 0.55),
		d("LNG", "power generation", "used_by", "LNG", "power generation", "LNG peaker plants.", 0.62, 0.50),
	)

	// --- critical minerals ---------------------------------------------------
	add(
		d("Chile", "lithium", "exports", "lithium", "", "Chilean brine lithium.", 0.88, 0.75),
		d("Argentina", "lithium", "exports", "lithium", "", "Argentine lithium triangle.", 0.62, 0.48),
		d("Australia", "lithium", "exports", "lithium", "", "Australian spodumene.", 0.72, 0.58),
		d("China", "lithium", "exports", "lithium", "", "Chinese lithium refining.", 0.68, 0.55),
		d("Democratic Republic of the Congo", "cobalt", "exports", "cobalt", "", "DRC mined cobalt dominance.", 0.92, 0.78),
		d("China", "cobalt", "exports", "cobalt", "", "Chinese cobalt refining.", 0.55, 0.42),
		d("Indonesia", "nickel", "exports", "nickel", "", "Indonesian nickel processing.", 0.82, 0.68),
		d("Russia", "nickel", "exports", "nickel", "", "Russian nickel production.", 0.58, 0.45),
		d("Chile", "copper", "exports", "copper", "", "Chilean copper mining.", 0.78, 0.65),
		d("Peru", "copper", "exports", "copper", "", "Peruvian copper exports.", 0.65, 0.52),
		d("China", "rare earths", "exports", "rare earths", "", "Chinese rare-earth processing.", 0.90, 0.85),
		d("Australia", "rare earths", "exports", "rare earths", "", "Australian rare-earth mining.", 0.42, 0.32),
		d("lithium", "EV batteries", "used_by", "lithium", "EV batteries", "Battery cathode lithium.", 0.85, 0.72),
		d("cobalt", "EV batteries", "used_by", "cobalt", "EV batteries", "High-nickel cathodes use cobalt.", 0.82, 0.68),
		d("nickel", "EV batteries", "used_by", "nickel", "EV batteries", "Nickel-rich cathodes.", 0.80, 0.65),
		d("copper", "renewable energy", "used_by", "copper", "renewable energy", "Solar and wind wiring.", 0.75, 0.62),
		d("copper", "data centers", "used_by", "copper", "data centers", "Power and cooling infrastructure.", 0.62, 0.48),
		d("rare earths", "defense manufacturing", "used_by", "rare earths", "defense manufacturing", "Rare-earth magnets in defense.", 0.78, 0.65),
		d("rare earths", "renewable energy", "used_by", "rare earths", "renewable energy", "Wind turbine magnets.", 0.72, 0.58),
		d("China", "batteries", "exports", "batteries", "", "Chinese battery cell exports.", 0.82, 0.70),
		d("Korea, Rep.", "batteries", "exports", "batteries", "", "Korean battery giants.", 0.75, 0.62),
		d("batteries", "EV batteries", "used_by", "batteries", "EV batteries", "Pack integrators use cells.", 0.88, 0.75),
		d("Korea, Rep.", "EV batteries", "industry_dependency", "batteries", "EV batteries", "Korean EV battery sector.", 0.80, 0.68),
		d("China", "EV batteries", "industry_dependency", "batteries", "EV batteries", "Chinese battery manufacturing.", 0.78, 0.65),
		d("lithium", "automotive electronics", "price_exposure", "lithium", "automotive electronics", "EV cost pass-through.", 0.55, 0.42),
	)

	// --- agriculture & food --------------------------------------------------
	add(
		d("Russia", "wheat", "exports", "wheat", "", "Russian wheat exports.", 0.82, 0.72),
		d("Ukraine", "wheat", "exports", "wheat", "", "Ukrainian wheat exports.", 0.75, 0.65),
		d("United States", "wheat", "exports", "wheat", "", "US grain exports.", 0.68, 0.55),
		d("Argentina", "wheat", "exports", "wheat", "", "Argentine wheat exports.", 0.58, 0.45),
		d("Brazil", "corn", "exports", "corn", "", "Brazilian corn exports.", 0.78, 0.65),
		d("United States", "corn", "exports", "corn", "", "US corn belt exports.", 0.72, 0.58),
		d("Ukraine", "corn", "exports", "corn", "", "Ukrainian corn exports.", 0.62, 0.50),
		d("India", "rice", "exports", "rice", "", "Indian rice exports.", 0.65, 0.52),
		d("Vietnam", "rice", "exports", "rice", "", "Vietnamese rice exports.", 0.58, 0.45),
		d("wheat", "China", "imports", "wheat", "", "Chinese wheat imports.", 0.55, 0.42),
		d("wheat", "Indonesia", "imports", "wheat", "", "Indonesian wheat demand.", 0.48, 0.38),
		d("wheat", "food security", "used_by", "wheat", "food security", "Wheat in national food systems.", 0.82, 0.70),
		d("corn", "China", "imports", "corn", "", "Chinese feed corn imports.", 0.52, 0.42),
		d("corn", "agriculture", "used_by", "corn", "agriculture", "Feed and ethanol demand.", 0.75, 0.62),
		d("rice", "China", "imports", "rice", "", "Chinese rice trade.", 0.42, 0.32),
		d("rice", "food security", "used_by", "rice", "food security", "Rice staple security.", 0.78, 0.65),
		d("Russia", "fertilizer", "exports", "fertilizer", "", "Russian fertilizer exports.", 0.80, 0.70),
		d("Canada", "fertilizer", "exports", "fertilizer", "", "Canadian potash exports.", 0.65, 0.52),
		d("fertilizer", "agriculture", "used_by", "fertilizer", "agriculture", "Crop yield depends on fertilizer.", 0.85, 0.72),
		d("fertilizer", "food security", "used_by", "fertilizer", "food security", "Fertilizer and harvest stability.", 0.78, 0.65),
		d("Ukraine", "agriculture", "industry_dependency", "wheat", "agriculture", "Ukrainian agri sector.", 0.72, 0.58),
		d("Brazil", "agriculture", "industry_dependency", "corn", "agriculture", "Brazilian agribusiness.", 0.78, 0.65),
		d("India", "agriculture", "industry_dependency", "rice", "agriculture", "Indian agricultural base.", 0.75, 0.62),
	)

	// --- industrial metals, solar, pharma, containers ------------------------
	add(
		d("China", "steel", "exports", "steel", "", "Chinese steel exports.", 0.78, 0.65),
		d("Russia", "steel", "exports", "steel", "", "Russian steel exports.", 0.55, 0.42),
		d("China", "aluminum", "exports", "aluminum", "", "Chinese aluminum smelting.", 0.72, 0.58),
		d("Canada", "aluminum", "exports", "aluminum", "", "Canadian aluminum.", 0.58, 0.45),
		d("steel", "construction", "used_by", "steel", "construction", "Structural steel demand.", 0.82, 0.70),
		d("steel", "energy-intensive manufacturing", "used_by", "steel", "energy-intensive manufacturing", "Industrial steel inputs.", 0.75, 0.62),
		d("aluminum", "aerospace manufacturing", "used_by", "aluminum", "aerospace manufacturing", "Airframe aluminum.", 0.68, 0.55),
		d("aluminum", "automotive electronics", "used_by", "aluminum", "automotive electronics", "Lightweight vehicle parts.", 0.55, 0.42),
		d("Canada", "uranium", "exports", "uranium", "", "Canadian uranium mining.", 0.62, 0.48),
		d("Australia", "uranium", "exports", "uranium", "", "Australian uranium exports.", 0.55, 0.42),
		d("uranium", "power generation", "used_by", "uranium", "power generation", "Nuclear fuel cycle.", 0.72, 0.58),
		d("China", "solar panels", "exports", "solar panels", "", "Chinese PV module exports.", 0.88, 0.82),
		d("solar panels", "renewable energy", "used_by", "solar panels", "renewable energy", "Utility-scale solar deployment.", 0.85, 0.72),
		d("India", "pharmaceuticals", "exports", "pharmaceuticals", "", "Indian generic pharma exports.", 0.72, 0.58),
		d("United States", "pharmaceuticals", "exports", "pharmaceuticals", "", "US innovative pharma.", 0.68, 0.55),
		d("China", "pharmaceuticals", "exports", "pharmaceuticals", "", "Chinese API exports.", 0.62, 0.48),
		d("pharmaceuticals", "medical supply chains", "used_by", "pharmaceuticals", "medical supply chains", "Hospital and pharmacy supply.", 0.88, 0.75),
		d("China", "shipping containers", "exports", "shipping containers", "", "Chinese container manufacturing.", 0.82, 0.72),
		d("shipping containers", "shipping logistics", "used_by", "shipping containers", "shipping logistics", "Containerised freight capacity.", 0.85, 0.72),
	)

	// --- routes --------------------------------------------------------------
	add(
		d("Strait of Hormuz", "crude oil", "route_exposure", "crude oil", "", "Gulf crude transit chokepoint.", 0.90, 0.85),
		d("Strait of Hormuz", "LNG", "route_exposure", "LNG", "", "Gulf LNG transit.", 0.75, 0.68),
		d("Suez Canal", "crude oil", "route_exposure", "crude oil", "", "Suez crude transit.", 0.65, 0.55),
		d("Suez Canal", "shipping containers", "route_exposure", "shipping containers", "", "Asia–Europe container route.", 0.72, 0.62),
		d("Red Sea", "crude oil", "route_exposure", "crude oil", "", "Red Sea shipping corridor.", 0.68, 0.58),
		d("Red Sea", "shipping containers", "route_exposure", "shipping containers", "", "Red Sea container routing.", 0.70, 0.58),
		d("Panama Canal", "shipping containers", "route_exposure", "shipping containers", "", "Panama container corridor.", 0.78, 0.68),
		d("Panama Canal", "LNG", "route_exposure", "LNG", "", "Atlantic–Pacific LNG routing.", 0.55, 0.45),
		d("Taiwan Strait", "semiconductors", "route_exposure", "semiconductors", "", "Chip shipping through Taiwan Strait.", 0.72, 0.62),
		d("South China Sea", "semiconductors", "route_exposure", "semiconductors", "", "Electronics trade corridor.", 0.75, 0.65),
		d("South China Sea", "shipping containers", "route_exposure", "shipping containers", "", "East Asian container flows.", 0.80, 0.70),
		d("Malacca Strait", "crude oil", "route_exposure", "crude oil", "", "Indian Ocean crude lane.", 0.72, 0.62),
		d("Malacca Strait", "LNG", "route_exposure", "LNG", "", "Asian LNG transit.", 0.65, 0.55),
		d("Black Sea", "wheat", "route_exposure", "wheat", "", "Black Sea grain corridor.", 0.78, 0.68),
		d("Black Sea", "fertilizer", "route_exposure", "fertilizer", "", "Black Sea fertilizer routing.", 0.62, 0.50),
		d("Black Sea", "corn", "route_exposure", "corn", "", "Black Sea corn exports route.", 0.68, 0.55),
	)

	// --- cross-sector and country links --------------------------------------
	add(
		d("Germany", "industrial machinery", "industry_dependency", "steel", "industrial machinery", "German machinery sector.", 0.78, 0.65),
		d("Japan", "industrial machinery", "industry_dependency", "semiconductors", "industrial machinery", "Japanese capital goods.", 0.72, 0.58),
		d("United States", "defense manufacturing", "industry_dependency", "semiconductors", "defense manufacturing", "US defense industrial base.", 0.75, 0.62),
		d("United States", "aerospace manufacturing", "industry_dependency", "aluminum", "aerospace manufacturing", "US aerospace supply chain.", 0.72, 0.58),
		d("Singapore", "shipping logistics", "industry_dependency", "shipping containers", "shipping logistics", "Singapore transshipment hub.", 0.82, 0.70),
		d("Netherlands", "shipping logistics", "industry_dependency", "crude oil", "shipping logistics", "Rotterdam port complex.", 0.75, 0.62),
		d("China", "renewable energy", "industry_dependency", "solar panels", "renewable energy", "Chinese renewable deployment.", 0.80, 0.68),
		d("India", "renewable energy", "industry_dependency", "solar panels", "renewable energy", "Indian solar build-out.", 0.65, 0.52),
		d("Germany", "power generation", "industry_dependency", "natural gas", "power generation", "German power sector.", 0.72, 0.58),
		d("Japan", "power generation", "industry_dependency", "LNG", "power generation", "Japanese power imports.", 0.78, 0.65),
		d("South Africa", "energy-intensive manufacturing", "industry_dependency", "copper", "energy-intensive manufacturing", "South African smelting.", 0.58, 0.45),
		d("Mexico", "construction", "industry_dependency", "steel", "construction", "Mexican construction demand.", 0.62, 0.48),
		d("Indonesia", "electronics manufacturing", "industry_dependency", "nickel", "electronics manufacturing", "Nickel-linked manufacturing.", 0.55, 0.42),
		d("Peru", "construction", "industry_dependency", "copper", "construction", "Peruvian copper-linked construction.", 0.52, 0.40),
		d("United Arab Emirates", "energy-intensive manufacturing", "industry_dependency", "crude oil", "energy-intensive manufacturing", "Gulf petrochemicals.", 0.68, 0.55),
		d("Australia", "renewable energy", "industry_dependency", "lithium", "renewable energy", "Australian battery supply chain.", 0.62, 0.48),
		d("consumer devices", "telecommunications", "depends_on", "semiconductors", "telecommunications", "Devices enable telecom services.", 0.65, 0.52),
		d("cloud infrastructure", "data centers", "depends_on", "semiconductors", "data centers", "Cloud runs in data centres.", 0.80, 0.68),
		d("EV batteries", "automotive electronics", "depends_on", "batteries", "automotive electronics", "EV platforms integrate power electronics.", 0.72, 0.58),
		d("medical supply chains", "food security", "depends_on", "pharmaceuticals", "food security", "Health and nutrition systems overlap.", 0.45, 0.35),
		d("United States", "medical supply chains", "industry_dependency", "pharmaceuticals", "medical supply chains", "US healthcare supply chain.", 0.75, 0.62),
		d("India", "medical supply chains", "industry_dependency", "pharmaceuticals", "medical supply chains", "Indian pharma distribution.", 0.68, 0.55),
		d("China", "telecommunications", "industry_dependency", "semiconductors", "telecommunications", "Chinese 5G rollout.", 0.78, 0.65),
		d("Korea, Rep.", "telecommunications", "industry_dependency", "semiconductors", "telecommunications", "Korean telecom equipment.", 0.65, 0.52),
		d("crude oil", "automotive electronics", "price_exposure", "crude oil", "automotive electronics", "Fuel and petrochemical pass-through.", 0.48, 0.38),
		d("natural gas", "energy-intensive manufacturing", "price_exposure", "natural gas", "energy-intensive manufacturing", "Gas price and industrial margins.", 0.62, 0.50),
		d("shipping containers", "consumer devices", "used_by", "shipping containers", "consumer devices", "Consumer goods shipped in containers.", 0.70, 0.55),
		d("shipping containers", "electronics manufacturing", "used_by", "shipping containers", "electronics manufacturing", "EMS imports via containers.", 0.75, 0.62),
		d("Russia", "Ukraine", "supplies", "fertilizer", "", "Regional fertilizer trade links.", 0.42, 0.32),
		d("China", "Australia", "depends_on", "lithium", "", "China refines Australian lithium.", 0.55, 0.42),
		d("Japan", "Australia", "depends_on", "LNG", "", "Japan imports Australian LNG.", 0.62, 0.50),
		d("Korea, Rep.", "Chile", "depends_on", "copper", "", "Korean industry copper demand.", 0.48, 0.38),
		d("Germany", "Russia", "depends_on", "natural gas", "", "Legacy European gas links.", 0.52, 0.42),
		d("India", "Saudi Arabia", "depends_on", "crude oil", "", "Indian crude from Gulf.", 0.58, 0.48),
		d("United States", "Taiwan", "depends_on", "semiconductors", "", "US reliance on Taiwan fabs.", 0.72, 0.65),
		d("Vietnam", "China", "depends_on", "semiconductors", "", "Vietnam EMS upstream links.", 0.62, 0.50),
		d("Netherlands", "United States", "depends_on", "LNG", "", "European LNG from US.", 0.45, 0.35),
		d("Singapore", "China", "depends_on", "shipping containers", "", "Transshipment from China.", 0.68, 0.55),
		d("United Arab Emirates", "India", "depends_on", "crude oil", "", "Gulf crude to India.", 0.55, 0.45),
		d("Brazil", "China", "depends_on", "corn", "", "Brazilian corn to China.", 0.52, 0.42),
		d("Argentina", "China", "depends_on", "lithium", "", "Lithium triangle to China.", 0.48, 0.38),
		d("Peru", "China", "depends_on", "copper", "", "Peruvian copper to China.", 0.58, 0.48),
		d("South Africa", "China", "depends_on", "cobalt", "", "Cobalt trade via refiners.", 0.42, 0.32),
		d("Democratic Republic of the Congo", "China", "depends_on", "cobalt", "", "DRC cobalt to Chinese refiners.", 0.75, 0.65),
		d("Chile", "China", "depends_on", "lithium", "", "Chilean lithium to China.", 0.68, 0.58),
		d("Ukraine", "Germany", "depends_on", "wheat", "", "Ukrainian grain to EU.", 0.45, 0.35),
		d("Russia", "China", "depends_on", "natural gas", "", "Pipeline gas to China.", 0.48, 0.38),
		d("Canada", "United States", "supplies", "natural gas", "", "Canadian gas to US.", 0.62, 0.52),
		d("Canada", "United States", "supplies", "crude oil", "", "Canadian crude to US refiners.", 0.55, 0.45),
		d("Mexico", "United States", "supplies", "semiconductors", "", "Nearshored auto electronics supply.", 0.58, 0.48),
		d("Taiwan", "Japan", "supplies", "semiconductors", "", "Taiwan chips to Japan.", 0.55, 0.45),
		d("Korea, Rep.", "United States", "supplies", "batteries", "", "Korean batteries to US EVs.", 0.52, 0.42),
		d("Germany", "China", "imports", "semiconductors", "", "German chip imports from Asia.", 0.48, 0.38),
		d("United States", "Japan", "imports", "semiconductors", "", "US–Japan chip trade.", 0.42, 0.32),
	)

	return out
}

func buildScenarios() []scenario {
	return []scenario{
		{
			ID: "taiwan_semiconductor_shock", Name: "Taiwan Semiconductor Export Collapse",
			Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse",
			ShockPercent: 30, Depth: 3,
			Description: "A 30% drop in Taiwan semiconductor exports and cascade through compute-dependent economies.",
		},
		{
			ID: "china_rare_earth_export_control", Name: "China Rare Earth Export Control",
			Source: "China", Commodity: "rare earths", ShockType: "export_collapse",
			ShockPercent: 35, Depth: 3,
			Description: "A 35% contraction in Chinese rare-earth exports hitting defense and renewable supply chains.",
		},
		{
			ID: "saudi_crude_oil_supply_cut", Name: "Saudi Crude Oil Supply Cut",
			Source: "Saudi Arabia", Commodity: "crude oil", ShockType: "supply_cut",
			ShockPercent: 25, Depth: 3,
			Description: "A 25% Saudi crude production cut pressuring importers and energy-intensive sectors.",
		},
		{
			ID: "russia_natural_gas_disruption", Name: "Russia Natural Gas Disruption",
			Source: "Russia", Commodity: "natural gas", ShockType: "supply_cut",
			ShockPercent: 40, Depth: 3,
			Description: "A 40% Russian natural gas supply disruption affecting European power and industry.",
		},
		{
			ID: "ukraine_wheat_export_disruption", Name: "Ukraine Wheat Export Disruption",
			Source: "Ukraine", Commodity: "wheat", ShockType: "export_collapse",
			ShockPercent: 45, Depth: 3,
			Description: "A 45% drop in Ukrainian wheat exports stressing food-security systems.",
		},
		{
			ID: "drc_cobalt_supply_disruption", Name: "DRC Cobalt Supply Disruption",
			Source: "Democratic Republic of the Congo", Commodity: "cobalt", ShockType: "supply_cut",
			ShockPercent: 30, Depth: 3,
			Description: "A 30% DRC cobalt supply disruption propagating into EV battery manufacturing.",
		},
		{
			ID: "chile_lithium_export_disruption", Name: "Chile Lithium Export Disruption",
			Source: "Chile", Commodity: "lithium", ShockType: "export_collapse",
			ShockPercent: 35, Depth: 3,
			Description: "A 35% Chilean lithium export shock hitting battery and automotive chains.",
		},
		{
			ID: "hormuz_crude_route_disruption", Name: "Strait of Hormuz Crude Route Disruption",
			Source: "Strait of Hormuz", Commodity: "crude oil", ShockType: "route_disruption",
			ShockPercent: 40, Depth: 3,
			Description: "A 40% reduction in crude oil transiting Hormuz and downstream import pressure.",
		},
		{
			ID: "panama_canal_shipping_disruption", Name: "Panama Canal Shipping Disruption",
			Source: "Panama Canal", Commodity: "shipping containers", ShockType: "route_disruption",
			ShockPercent: 35, Depth: 3,
			Description: "A 35% Panama Canal container capacity shock disrupting global logistics.",
		},
		{
			ID: "south_china_sea_electronics_disruption", Name: "South China Sea Electronics Disruption",
			Source: "South China Sea", Commodity: "semiconductors", ShockType: "route_disruption",
			ShockPercent: 30, Depth: 3,
			Description: "A 30% South China Sea semiconductor shipping disruption hitting electronics trade.",
		},
	}
}
