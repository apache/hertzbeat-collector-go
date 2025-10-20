/*package unit

 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package unit

import (
	"fmt"
	"strings"
)

// Conversion represents a unit conversion configuration
type Conversion struct {
	Field      string
	OriginUnit string
	NewUnit    string
}

// ParseConversion parses a unit conversion string
// Format: "fieldName=originUnit->newUnit"
// Example: "committed=B->MB"
func ParseConversion(unitStr string) (*Conversion, error) {
	// Split by '='
	parts := strings.Split(unitStr, "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid unit conversion format: %s, expected 'fieldName=originUnit->newUnit'", unitStr)
	}

	fieldName := strings.TrimSpace(parts[0])
	conversionPart := strings.TrimSpace(parts[1])

	// Split by '->'
	unitParts := strings.Split(conversionPart, "->")
	if len(unitParts) != 2 {
		return nil, fmt.Errorf("invalid unit conversion format: %s, expected 'originUnit->newUnit'", conversionPart)
	}

	return &Conversion{
		Field:      fieldName,
		OriginUnit: strings.TrimSpace(unitParts[0]),
		NewUnit:    strings.TrimSpace(unitParts[1]),
	}, nil
}

// Convert converts a value from origin unit to new unit
// Supports common unit conversions for bytes, bits, and time
func Convert(value float64, originUnit, newUnit string) (float64, error) {
	// Normalize unit names to uppercase for comparison
	originUnit = strings.ToUpper(originUnit)
	newUnit = strings.ToUpper(newUnit)

	// If units are the same, no conversion needed
	if originUnit == newUnit {
		return value, nil
	}

	// Define unit conversion factors (all relative to base unit)
	// For bytes: base unit is B (byte)
	byteUnits := map[string]float64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
		"PB": 1024 * 1024 * 1024 * 1024 * 1024,
		// Binary versions
		"KIB": 1024,
		"MIB": 1024 * 1024,
		"GIB": 1024 * 1024 * 1024,
		"TIB": 1024 * 1024 * 1024 * 1024,
		"PIB": 1024 * 1024 * 1024 * 1024 * 1024,
	}

	// For bits: base unit is b (bit)
	bitUnits := map[string]float64{
		"B":    1,
		"KB":   1000,
		"MB":   1000 * 1000,
		"GB":   1000 * 1000 * 1000,
		"TB":   1000 * 1000 * 1000 * 1000,
		"PB":   1000 * 1000 * 1000 * 1000 * 1000,
		"KBIT": 1000,
		"MBIT": 1000 * 1000,
		"GBIT": 1000 * 1000 * 1000,
		"TBIT": 1000 * 1000 * 1000 * 1000,
		"PBIT": 1000 * 1000 * 1000 * 1000 * 1000,
	}

	// For time: base unit is ns (nanosecond)
	timeUnits := map[string]float64{
		"NS":  1,
		"US":  1000,
		"MS":  1000_000,
		"S":   1000_000_000,
		"MIN": 60_000_000_000,
		"H":   3600_000_000_000,
		"D":   86_400_000_000_000,
	}

	// Try byte conversion first
	if originFactor, originOk := byteUnits[originUnit]; originOk {
		if newFactor, newOk := byteUnits[newUnit]; newOk {
			return value * originFactor / newFactor, nil
		}
	}

	// Try bit conversion
	if originFactor, originOk := bitUnits[originUnit]; originOk {
		if newFactor, newOk := bitUnits[newUnit]; newOk {
			return value * originFactor / newFactor, nil
		}
	}

	// Try time conversion
	if originFactor, originOk := timeUnits[originUnit]; originOk {
		if newFactor, newOk := timeUnits[newUnit]; newOk {
			return value * originFactor / newFactor, nil
		}
	}

	return 0, fmt.Errorf("unsupported unit conversion from %s to %s", originUnit, newUnit)
}
