package ancillary

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/adamf/jetway/pkg/pnr"
)

// list is a price list of the shape the large United States carriers
// publish. The bag prices are the ones American put in force on 9 April
// 2026 as consumer groups reported them: a first checked bag at 50
// dollars at the airport and 45 paid in advance, a second at 60 and 55.
// The other lines are of the kinds those carriers have: bags free in the
// front cabins and for a top tier, a third bag at a high price, a seat
// with more legroom priced by distance, the internet free to members and
// sold to the rest, a snack box sold on longer flights only, and a drink
// free in the front cabin and sold behind.
func list() *Catalogue {
	usd := func(dollars int64) (int64, string) { return dollars * 100, "USD" }
	rule := func(service string, dollars int64, r Rule) Rule {
		r.Service = service
		r.Amount, r.Currency = usd(dollars)
		return r
	}
	return &Catalogue{Carrier: "XX",
		Services: []Service{
			{Code: "BAG", RFIC: pnr.RFICBaggage, Name: "checked bag", PerUnit: true},
			{Code: "LEG", RFIC: pnr.RFICAir, Name: "seat with more legroom"},
			{Code: "NET", RFIC: pnr.RFICInflight, Name: "internet"},
			{Code: "BOX", RFIC: pnr.RFICInflight, Name: "snack box", PerUnit: true},
			{Code: "DRK", RFIC: pnr.RFICInflight, Name: "alcoholic drink", PerUnit: true},
			{Code: "PET", RFIC: pnr.RFICBaggage, Name: "pet in the cabin"},
		},
		Rules: []Rule{
			// Bags. The front cabins have 2 free. A gold member has the
			// first free. Then the price by unit and by moment.
			// Nothing for the hold is sold in the air.
			{Service: "BAG", Points: []Point{OnBoard}, NotOffered: true},
			{Service: "BAG", Cabins: []string{"F", "C"}, ToUnit: 2, Included: true},
			{Service: "BAG", Tiers: []string{"gold"}, ToUnit: 1, Included: true},
			rule("BAG", 45, Rule{FromUnit: 1, ToUnit: 1, Points: []Point{AtBooking}}),
			rule("BAG", 50, Rule{FromUnit: 1, ToUnit: 1}),
			rule("BAG", 55, Rule{FromUnit: 2, ToUnit: 2, Points: []Point{AtBooking}}),
			rule("BAG", 60, Rule{FromUnit: 2, ToUnit: 2}),
			rule("BAG", 200, Rule{FromUnit: 3}),
			// More legroom: only in the main cabin, by distance, before
			// the door.
			{Service: "LEG", Cabins: []string{"F", "C"}, NotOffered: true},
			rule("LEG", 29, Rule{MaxKM: 1000, Points: []Point{AtBooking, AtAirport, AtGate}}),
			rule("LEG", 59, Rule{MinKM: 1001, MaxKM: 3000, Points: []Point{AtBooking, AtAirport, AtGate}}),
			rule("LEG", 119, Rule{MinKM: 3001, Points: []Point{AtBooking, AtAirport, AtGate}}),
			// The internet: free to any member, 10 dollars to the rest,
			// and only in the air.
			{Service: "NET", Tiers: []string{"member", "gold"}, Points: []Point{OnBoard}, Included: true},
			rule("NET", 10, Rule{Points: []Point{OnBoard}}),
			// A snack box: on flights of 3 hours and more.
			rule("BOX", 10, Rule{MinMinutes: 180, Points: []Point{OnBoard}}),
			// A drink: free in front, 11 dollars behind, more on a peak day.
			{Service: "DRK", Cabins: []string{"F", "C"}, Points: []Point{OnBoard}, Included: true},
			rule("DRK", 12, Rule{Points: []Point{OnBoard}, Peak: Yes}),
			rule("DRK", 11, Rule{Points: []Point{OnBoard}}),
			// A pet: domestic itineraries only, arranged before the gate.
			{Service: "PET", International: Yes, NotOffered: true},
			rule("PET", 150, Rule{Points: []Point{AtBooking, AtAirport}}),
		}}
}

func TestPriceListReadByFirstMatch(t *testing.T) {
	c := list()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	main := Passenger{Cabin: "Y", Class: "Q", KM: 1500, Minutes: 150}
	at := func(p Passenger, pt Point) Passenger { p.Point = pt; return p }
	front := Passenger{Cabin: "C", Class: "J", KM: 1500, Minutes: 150}
	gold := Passenger{Cabin: "Y", Class: "Q", Tier: "gold", KM: 1500, Minutes: 150}
	member := Passenger{Cabin: "Y", Class: "Q", Tier: "member", KM: 1500, Minutes: 150}
	for _, c2 := range []struct {
		name     string
		code     string
		p        Passenger
		unit     int
		dollars  int64
		included bool
		offered  bool
	}{
		{"first bag at the airport", "BAG", at(main, AtAirport), 1, 50, false, true},
		{"first bag paid in advance", "BAG", at(main, AtBooking), 1, 45, false, true},
		{"second bag at the airport", "BAG", at(main, AtAirport), 2, 60, false, true},
		{"second bag paid in advance", "BAG", at(main, AtBooking), 2, 55, false, true},
		{"third bag", "BAG", at(main, AtAirport), 3, 200, false, true},
		{"fifth bag", "BAG", at(main, AtBooking), 5, 200, false, true},
		{"first bag in business", "BAG", at(front, AtAirport), 1, 0, true, true},
		{"second bag in business", "BAG", at(front, AtAirport), 2, 0, true, true},
		{"third bag in business", "BAG", at(front, AtAirport), 3, 200, false, true},
		{"a gold member's first bag", "BAG", at(gold, AtAirport), 1, 0, true, true},
		{"a gold member's second bag", "BAG", at(gold, AtAirport), 2, 60, false, true},
		{"a member's first bag", "BAG", at(member, AtAirport), 1, 50, false, true},
		{"legroom on 1,500 km", "LEG", at(main, AtBooking), 1, 59, false, true},
		{"legroom on 1,000 km", "LEG", at(Passenger{Cabin: "Y", KM: 1000}, AtBooking), 1, 29, false, true},
		{"legroom on 3,001 km", "LEG", at(Passenger{Cabin: "Y", KM: 3001}, AtBooking), 1, 119, false, true},
		{"legroom in business", "LEG", at(front, AtBooking), 1, 0, false, false},
		{"a second seat with legroom", "LEG", at(main, AtBooking), 2, 0, false, false},
		{"the internet for a member", "NET", at(member, OnBoard), 1, 0, true, true},
		{"the internet for a stranger", "NET", at(main, OnBoard), 1, 10, false, true},
		{"the internet on the ground", "NET", at(main, AtAirport), 1, 0, false, false},
		{"a snack box on 150 minutes", "BOX", at(main, OnBoard), 1, 0, false, false},
		{"a snack box on 180 minutes", "BOX", at(Passenger{Cabin: "Y", Minutes: 180}, OnBoard), 1, 10, false, true},
		{"a drink behind", "DRK", at(main, OnBoard), 1, 11, false, true},
		{"a drink in front", "DRK", at(front, OnBoard), 1, 0, true, true},
		{"a pet at home", "PET", at(main, AtBooking), 1, 150, false, true},
		{"a pet abroad", "PET", at(Passenger{Cabin: "Y", International: true}, AtBooking), 1, 0, false, false},
		{"a service the carrier does not have", "SPA", at(main, AtBooking), 1, 0, false, false},
	} {
		q, ok := c.Price(c2.code, c2.p, c2.unit)
		if ok != c2.offered {
			t.Errorf("%s: offered %v, want %v", c2.name, ok, c2.offered)
			continue
		}
		if !ok {
			continue
		}
		if q.Amount != c2.dollars*100 || q.Included != c2.included || q.Code != c2.code || q.Unit != c2.unit {
			t.Errorf("%s: %+v; want %d dollars, included %v", c2.name, q, c2.dollars, c2.included)
		}
		if !q.Included && q.Currency != "USD" {
			t.Errorf("%s: currency %q", c2.name, q.Currency)
		}
	}
	// A drink on a peak day.
	peak := at(main, OnBoard)
	peak.Peak = true
	if q, ok := c.Price("DRK", peak, 1); !ok || q.Amount != 1200 {
		t.Errorf("a drink on a peak day: %+v %v, want 12 dollars", q, ok)
	}

	// 3 bags at the airport cost 50 + 60 + 200. A gold member's 2 cost 60.
	if amount, sold, cur := c.Total("BAG", at(main, AtAirport), 3); amount != 31000 || sold != 3 || cur != "USD" {
		t.Errorf("3 bags: %d cents for %d, want 31000 for 3", amount, sold)
	}
	if amount, sold, _ := c.Total("BAG", at(gold, AtAirport), 2); amount != 6000 || sold != 2 {
		t.Errorf("a gold member's 2 bags: %d cents for %d, want 6000 for 2", amount, sold)
	}
	if amount, sold, _ := c.Total("LEG", at(main, AtBooking), 2); amount != 5900 || sold != 1 {
		t.Errorf("2 seats with legroom for 1 passenger: %d cents for %d, want 5900 for 1", amount, sold)
	}

	// The menu on board a flight of 4 hours, for a passenger in the main
	// cabin who is not a member: the internet, a snack box and a drink.
	// Bags, seats and pets have no price in the air and are not on it.
	menu := func(p Passenger) string {
		var codes []string
		for _, q := range c.Menu(p) {
			codes = append(codes, q.Code)
		}
		return strings.Join(codes, " ")
	}
	if got := menu(Passenger{Cabin: "Y", Minutes: 240, Point: OnBoard}); got != "NET BOX DRK" {
		t.Errorf("the menu on board: %s", got)
	}
	// On 2 hours there is no snack box. At booking there are bags, the
	// seat and the pet.
	if got := menu(Passenger{Cabin: "Y", Minutes: 120, Point: OnBoard}); got != "NET DRK" {
		t.Errorf("the menu on board a short flight: %s", got)
	}
	if got := menu(Passenger{Cabin: "Y", KM: 800, Minutes: 120, Point: AtBooking}); got != "BAG LEG PET" {
		t.Errorf("the menu at booking: %s", got)
	}
}

// A price list is data. It goes to JSON and comes back the same, with
// its bounds.
func TestCatalogueRoundTrip(t *testing.T) {
	c := list()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back Catalogue
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, &back) {
		t.Errorf("the price list changed on its way through JSON:\n%s", b)
	}
	if q, ok := back.Price("LEG", Passenger{Cabin: "Y", KM: 3001, Point: AtBooking}, 1); !ok || q.Amount != 11900 {
		t.Errorf("after the round trip, legroom on 3,001 km: %+v %v", q, ok)
	}
}

func TestCatalogueValidate(t *testing.T) {
	for name, c := range map[string]*Catalogue{
		"no code":        {Services: []Service{{RFIC: pnr.RFICBaggage, Name: "bag"}}},
		"code twice":     {Services: []Service{{Code: "BAG", RFIC: pnr.RFICBaggage}, {Code: "BAG", RFIC: pnr.RFICBaggage}}},
		"no such reason": {Services: []Service{{Code: "BAG", RFIC: "Z"}}},
		"no such service": {Services: []Service{{Code: "BAG", RFIC: pnr.RFICBaggage}},
			Rules: []Rule{{Service: "NET", Amount: 100}}},
		"price below zero": {Services: []Service{{Code: "BAG", RFIC: pnr.RFICBaggage}},
			Rules: []Rule{{Service: "BAG", Amount: -1}}},
		"free and priced": {Services: []Service{{Code: "BAG", RFIC: pnr.RFICBaggage}},
			Rules: []Rule{{Service: "BAG", Included: true, Amount: 100}}},
		"free and not sold": {Services: []Service{{Code: "BAG", RFIC: pnr.RFICBaggage}},
			Rules: []Rule{{Service: "BAG", Included: true, NotOffered: true}}},
	} {
		if err := c.Validate(); err == nil {
			t.Errorf("%s: the catalogue passed", name)
		}
	}
}
