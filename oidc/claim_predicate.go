package oidc

import (
	// "reflect"

	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ```
// - and:
//   - or:
//     - claim_key_str: "value"
//     - claim_key_int: 3
//     - claim_key_bool: true
// 	- claim_key_float: 3.14
// 	- claim_key_array:
// 	  - "value1"
// 	  - "value2"
//  - claim_key_b: "blah"
// ```

type ClaimPredicate interface {
	Validate(input jwt.MapClaims) bool
	String() string
}

type PredicateComposer interface {
	And(predicates ...ClaimPredicate) ClaimPredicate
	Or(predicates ...ClaimPredicate) ClaimPredicate
}

type Combinator func(...ClaimPredicate) ClaimPredicate

// And combines the children with an AND
func And(children ...ClaimPredicate) ClaimPredicate {
	return &andPredicate{Children: children}
}

// Or combines the children with an OR
func Or(children ...ClaimPredicate) ClaimPredicate {
	return &orPredicate{Children: children}
}

type andPredicate struct {
	Children []ClaimPredicate
}

func (a *andPredicate) Validate(claims jwt.MapClaims) bool {
	if len(a.Children) == 0 {
		return true
	}

	for _, child := range a.Children {
		if !child.Validate(claims) {
			return false
		}
	}

	return true
}

func StringifyList[T fmt.Stringer](list []T) []string {
	result := make([]string, 0, len(list))
	for _, item := range list {
		result = append(result, item.String())
	}
	return result
}

func (a *andPredicate) String() string {
	childrenStrings := StringifyList(a.Children)
	return strings.Join(childrenStrings, " AND ")
}

type orPredicate struct {
	Children []ClaimPredicate
}

func (o *orPredicate) Validate(claims jwt.MapClaims) bool {
	if len(o.Children) == 0 {
		return false
	}

	for _, child := range o.Children {
		if child.Validate(claims) {
			return true
		}
	}

	return false
}

func (o *orPredicate) String() string {
	childrenStrings := StringifyList(o.Children)
	return strings.Join(childrenStrings, " OR ")
}

// ClaimKey is a claim key predicate
type ClaimKey struct {
	Key   string
	Value interface{}
}

// Validate validates the input against the claim key
// If the value is a string, and the claim is a string it will check if the values are equal
// If the value is a string, and the claim is a list it will check if the value is in the list
func (c *ClaimKey) Validate(claims jwt.MapClaims) bool {
	if v, ok := claims[c.Key]; ok {
		switch v := v.(type) {
		case []interface{}:
			for _, item := range v {
				if item == c.Value {
					return true
				}
			}
			return false
		default:
			return v == c.Value
		}
	}
	return false
}

func (c *ClaimKey) String() string {
	return fmt.Sprintf("%s == %v", c.Key, c.Value)
}

func parseClaimPredicateList(predicateList []interface{}, combine Combinator) ClaimPredicate {
	result := make([]ClaimPredicate, 0, len(predicateList))
	for _, predicate := range predicateList {
		if p, ok := predicate.(map[string]interface{}); ok {
			result = append(result, ParseClaimPredicates(p))
		}
	}
	return combine(result...)
}

type staticPredicate struct {
	result bool
}

func (p *staticPredicate) Validate(_ jwt.MapClaims) bool {
	return p.result
}

func (p *staticPredicate) String() string {
	return fmt.Sprintf("%v", p.result)
}

// ParseClaimPredicates parses the input into a claim predicate
// The input can be a map[string]interface{} or a []map[string]interface{}
func ParseClaimPredicates(input interface{}) ClaimPredicate {
	switch v := input.(type) {
	case map[string]interface{}:
		return parseClaimPredicateMap(v)
	case []interface{}:
		return parseClaimPredicateList(v, And)
	default:
		return &staticPredicate{result: true}
	}
}

func parseClaimPredicateMap(predicateMap map[string]interface{}) ClaimPredicate {
	if len(predicateMap) == 0 {
		return &staticPredicate{result: true}
	}

	predicates := make([]ClaimPredicate, 0, len(predicateMap))

	for key, value := range predicateMap {
		if key == "and" {
			if children, ok := value.([]interface{}); ok {
				predicates = append(predicates, parseClaimPredicateList(children, And))
			}
		} else if key == "or" {
			if children, ok := value.([]interface{}); ok {
				predicates = append(predicates, parseClaimPredicateList(children, Or))
			}
		} else {
			predicates = append(predicates, &ClaimKey{
				Key:   key,
				Value: value,
			})
		}
	}

	return And(predicates...)
}
