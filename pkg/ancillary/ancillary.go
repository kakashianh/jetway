// Package ancillary is a carrier's optional services: what it sells beside
// the seat, to which passenger, at which moment, and for how much.
//
// A fare buys carriage. A checked bag, a seat chosen in advance, a seat
// with more legroom, a meal, a connection to the internet or a blanket is
// an ancillary service: 14 CFR 260.2 calls it an optional service for a
// fee beyond the transportation. A carrier's price list for these is a
// catalogue. It says what each service costs a given passenger, and also
// where the service is part of the fare or of the passenger's standing
// and costs nothing, and where it is not sold at all.
//
// The shape is the one the industry files optional services in: a list of
// services, each with a reason for issuance code and a sub code, and for
// each a sequence of rules that are tried in order. The first rule that
// matches the passenger decides. That is how a filed optional service
// record is read, and it lets a price list say "free in the front cabin,
// free for gold members, 45 when prepaid, 50 at the airport" in that
// order without any rule knowing about the others.
//
// The sub codes themselves belong to whoever files them. This package
// carries none: a caller's catalogue brings its own, the industry's where
// it has them. What is sold becomes a coupon of an electronic
// miscellaneous document (pkg/pnr, pkg/gateway), which Quote.RFIC and
// Quote.Code are for.
package ancillary

import (
	"fmt"
	"strings"

	"github.com/adamf/jetway/pkg/pnr"
)

// Point is the moment a service is bought. Carriers price by it: a bag
// costs less paid for in advance than at the counter, and most at the
// gate.
type Point string

const (
	// AtBooking is with the ticket, or any time before the airport.
	AtBooking Point = "booking"
	// AtAirport is at check-in.
	AtAirport Point = "airport"
	// AtGate is at the gate, after check-in has closed for the service.
	AtGate Point = "gate"
	// OnBoard is in the air.
	OnBoard Point = "onboard"
)

// Service is one thing a carrier sells beside the seat.
type Service struct {
	// Code is the sub code that says what was bought: the reason for
	// issuance sub code on the document's coupon.
	Code string `json:"code"`
	// RFIC is the reason for issuance code the sub code belongs to.
	RFIC pnr.RFIC `json:"rfic"`
	Name string   `json:"name"`
	// PerUnit says the price is for each unit, a bag or a drink, and a
	// passenger may buy several. Otherwise the passenger buys it once for
	// the flight.
	PerUnit bool `json:"per_unit,omitempty"`
}

// Passenger is what a price list needs to know about the passenger, the
// flight and the moment. A zero field is unknown, and a rule that asks
// for a value does not match an unknown one.
type Passenger struct {
	// Cabin is the compartment: F, C or Y.
	Cabin string
	// Class is the booking class, and Brand the fare product where the
	// carrier sells by product.
	Class, Brand string
	// Tier is the passenger's standing with the carrier, in the carrier's
	// own words: a loyalty tier, a card, a membership.
	Tier string
	// International says the itinerary crosses a border.
	International bool
	// KM is the flight's distance and Minutes its scheduled time.
	KM, Minutes int
	// Peak says the carrier counts the date as a peak one.
	Peak bool
	// Point is when the service is bought.
	Point Point
}

// Rule is one line of a price list for one service. Every condition that
// is set must hold. An empty list or a zero bound is no condition.
type Rule struct {
	Service string `json:"service"`

	Cabins  []string `json:"cabins,omitempty"`
	Classes []string `json:"classes,omitempty"`
	Brands  []string `json:"brands,omitempty"`
	Tiers   []string `json:"tiers,omitempty"`
	Points  []Point  `json:"points,omitempty"`
	// International, when set, wants an itinerary that is or is not
	// international. Peak likewise.
	International *bool `json:"international,omitempty"`
	Peak          *bool `json:"peak,omitempty"`
	// MinKM and MaxKM bound the flight's distance, and MinMinutes and
	// MaxMinutes its time. Zero is no bound. The bounds are inclusive.
	MinKM      int `json:"min_km,omitempty"`
	MaxKM      int `json:"max_km,omitempty"`
	MinMinutes int `json:"min_minutes,omitempty"`
	MaxMinutes int `json:"max_minutes,omitempty"`
	// FromUnit and ToUnit bound which unit this is, for a service sold by
	// the unit: 1 and 1 for the first bag, 3 and 0 for the third and each
	// after it. Zero is no bound.
	FromUnit int `json:"from_unit,omitempty"`
	ToUnit   int `json:"to_unit,omitempty"`

	// The outcome. Included is free to this passenger: the fare or the
	// passenger's standing covers it. NotOffered is not for sale to this
	// passenger. Otherwise the price is Amount, in the currency's minor
	// units.
	Included   bool   `json:"included,omitempty"`
	NotOffered bool   `json:"not_offered,omitempty"`
	Amount     int64  `json:"amount,omitempty"`
	Currency   string `json:"currency,omitempty"`
}

// Catalogue is a carrier's services and its price list for them. The
// rules for a service are tried in the order they are listed.
type Catalogue struct {
	Carrier  string    `json:"carrier"`
	Services []Service `json:"services"`
	Rules    []Rule    `json:"rules"`
}

// Quote is the answer for one service and one passenger.
type Quote struct {
	Service
	// Unit is which unit was asked for, 1 for a service bought once.
	Unit int `json:"unit"`
	// Included says the service costs this passenger nothing. Amount is
	// then zero.
	Included bool   `json:"included,omitempty"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency,omitempty"`
}

// Validate reports the first thing wrong with a catalogue: a service with
// no code or a code used twice, a reason for issuance code that is not
// one, a rule for a service the catalogue does not have, or a rule with a
// price below zero or with 2 outcomes.
func (c *Catalogue) Validate() error {
	seen := map[string]bool{}
	for _, s := range c.Services {
		switch {
		case s.Code == "":
			return fmt.Errorf("ancillary: service %q has no code", s.Name)
		case seen[s.Code]:
			return fmt.Errorf("ancillary: code %s is used twice", s.Code)
		case s.RFIC.Meaning() == "":
			return fmt.Errorf("ancillary: service %s has reason for issuance code %q, which is not one", s.Code, s.RFIC)
		}
		seen[s.Code] = true
	}
	for i, r := range c.Rules {
		switch {
		case !seen[r.Service]:
			return fmt.Errorf("ancillary: rule %d is for service %q, which the catalogue does not have", i+1, r.Service)
		case r.Amount < 0:
			return fmt.Errorf("ancillary: rule %d for %s has a price below zero", i+1, r.Service)
		case r.Included && r.NotOffered, (r.Included || r.NotOffered) && r.Amount != 0:
			return fmt.Errorf("ancillary: rule %d for %s has more than 1 outcome", i+1, r.Service)
		}
	}
	return nil
}

// service finds a service by its code.
func (c *Catalogue) service(code string) (Service, bool) {
	for _, s := range c.Services {
		if s.Code == code {
			return s, true
		}
	}
	return Service{}, false
}

// Price is what a service costs a passenger, for the unit asked: 1 for
// the first bag, 2 for the second. ok is false when the carrier does not
// sell the service to this passenger: it has no such service, a rule says
// so, or no rule matches. A service with no rule that matches is not for
// sale, because a price list that is silent has not set a price.
func (c *Catalogue) Price(code string, p Passenger, unit int) (q Quote, ok bool) {
	s, found := c.service(code)
	if !found {
		return Quote{}, false
	}
	if unit < 1 {
		unit = 1
	}
	if !s.PerUnit && unit > 1 {
		return Quote{}, false
	}
	for _, r := range c.Rules {
		if r.Service != code || !r.matches(p, unit) {
			continue
		}
		if r.NotOffered {
			return Quote{}, false
		}
		return Quote{Service: s, Unit: unit, Included: r.Included, Amount: r.Amount, Currency: r.Currency}, true
	}
	return Quote{}, false
}

// Menu is every service the carrier sells this passenger at this moment,
// each at the price of its first unit, in the catalogue's order. A
// service that is included is on the menu at no charge.
func (c *Catalogue) Menu(p Passenger) []Quote {
	var out []Quote
	for _, s := range c.Services {
		if q, ok := c.Price(s.Code, p, 1); ok {
			out = append(out, q)
		}
	}
	return out
}

// Total is what a number of units of a service cost a passenger together,
// each at its own unit's price, and how many of them the carrier sells.
// A passenger with 3 bags pays the first, the second and the third bag's
// prices.
func (c *Catalogue) Total(code string, p Passenger, units int) (amount int64, sold int, currency string) {
	for u := 1; u <= units; u++ {
		q, ok := c.Price(code, p, u)
		if !ok {
			break
		}
		amount += q.Amount
		sold++
		if q.Currency != "" {
			currency = q.Currency
		}
	}
	return amount, sold, currency
}

func (r Rule) matches(p Passenger, unit int) bool {
	switch {
	case !in(r.Cabins, p.Cabin), !in(r.Classes, p.Class), !in(r.Brands, p.Brand), !in(r.Tiers, p.Tier):
		return false
	case r.International != nil && *r.International != p.International:
		return false
	case r.Peak != nil && *r.Peak != p.Peak:
		return false
	case r.MinKM > 0 && p.KM < r.MinKM, r.MaxKM > 0 && p.KM > r.MaxKM:
		return false
	case r.MinMinutes > 0 && p.Minutes < r.MinMinutes, r.MaxMinutes > 0 && p.Minutes > r.MaxMinutes:
		return false
	case r.FromUnit > 0 && unit < r.FromUnit, r.ToUnit > 0 && unit > r.ToUnit:
		return false
	}
	if len(r.Points) > 0 {
		found := false
		for _, pt := range r.Points {
			if pt == p.Point {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// in reports whether a value is one of a list. An empty list takes any
// value. An unknown value, which is empty, is in no list of names.
func in(list []string, v string) bool {
	if len(list) == 0 {
		return true
	}
	for _, x := range list {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}

// Yes and No are for a rule's International and Peak conditions.
var (
	yes, no = true, false
	Yes     = &yes
	No      = &no
)
