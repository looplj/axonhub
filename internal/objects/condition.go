package objects

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

type ConditionType string

const (
	ConditionTypeCondition ConditionType = "condition"
	ConditionTypeGroup     ConditionType = "group"
)

type Condition struct {
	Type       ConditionType `json:"type"`
	Logic      string        `json:"logic,omitempty"`
	Conditions []Condition   `json:"conditions,omitempty"`
	Field      string        `json:"field,omitempty"`
	Operator   string        `json:"operator,omitempty"`
	Value      any           `json:"value,omitempty"`
}

func (c *Condition) UnmarshalJSON(data []byte) error {
	type alias Condition

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value alias
	if err := decoder.Decode(&value); err != nil {
		return err
	}

	value.Value = normalizeJSONNumberValue(value.Value)
	*c = Condition(value)

	return nil
}

var compiledConditionCache sync.Map // map[string]*vm.Program

//nolint:forcetypeassert // Checked.
func compileCondition(expression string) (*vm.Program, error) {
	if cached, ok := compiledConditionCache.Load(expression); ok {
		return cached.(*vm.Program), nil
	}

	program, err := expr.Compile(expression, expr.AsBool())
	if err != nil {
		return nil, err
	}

	compiledConditionCache.Store(expression, program)

	return program, nil
}

func Evaluate(condition Condition, data any) bool {
	expression, err := ToExpr(condition)
	if err != nil {
		return false
	}

	program, err := compileCondition(expression)
	if err != nil {
		return false
	}

	output, err := expr.Run(program, data)
	if err != nil {
		return false
	}

	result, ok := output.(bool)

	return ok && result
}

func ToExpr(condition Condition) (string, error) {
	nodeType := condition.Type
	if nodeType == "" {
		nodeType = ConditionTypeGroup
	}

	if nodeType == ConditionTypeCondition {
		return conditionToExpr(condition)
	}

	if len(condition.Conditions) == 0 {
		return "true", nil
	}

	logic := normalizeLogic(condition.Logic)

	parts := make([]string, 0, len(condition.Conditions))
	for _, child := range condition.Conditions {
		part, err := nodeToExpr(child)
		if err != nil {
			return "", err
		}

		parts = append(parts, part)
	}

	return "(" + strings.Join(parts, " "+logic+" ") + ")", nil
}

func normalizeLogic(logic string) string {
	switch strings.ToUpper(strings.TrimSpace(logic)) {
	case "OR":
		return "||"
	default:
		return "&&"
	}
}

func nodeToExpr(condition Condition) (string, error) {
	switch condition.Type {
	case "", ConditionTypeCondition:
		return conditionToExpr(condition)
	case ConditionTypeGroup:
		return ToExpr(condition)
	default:
		return "", fmt.Errorf("unsupported condition type %q", condition.Type)
	}
}

func conditionToExpr(condition Condition) (string, error) {
	field := strings.TrimSpace(condition.Field)
	if field == "" {
		return "", fmt.Errorf("field is required")
	}

	operator := normalizeOperator(condition.Operator)
	if operator == "" {
		return "", fmt.Errorf("unsupported operator %q", condition.Operator)
	}

	valueExpr, err := literalExpr(condition.Value)
	if err != nil {
		return "", err
	}

	return field + " " + operator + " " + valueExpr, nil
}

func normalizeOperator(operator string) string {
	switch strings.TrimSpace(strings.ToLower(operator)) {
	case "lt", "<":
		return "<"
	case "lte", "<=":
		return "<="
	case "gt", ">":
		return ">"
	case "gte", ">=":
		return ">="
	case "eq", "=", "==":
		return "=="
	case "ne", "!=", "<>":
		return "!="
	default:
		return ""
	}
}

func literalExpr(value any) (string, error) {
	if value == nil {
		return "", fmt.Errorf("value is required")
	}

	switch v := value.(type) {
	case string:
		return fmt.Sprintf("%q", v), nil
	case bool:
		if v {
			return "true", nil
		}

		return "false", nil
	case int:
		return fmt.Sprintf("%d", v), nil
	case int8:
		return fmt.Sprintf("%d", v), nil
	case int16:
		return fmt.Sprintf("%d", v), nil
	case int32:
		return fmt.Sprintf("%d", v), nil
	case int64:
		return fmt.Sprintf("%d", v), nil
	case uint:
		return fmt.Sprintf("%d", v), nil
	case uint8:
		return fmt.Sprintf("%d", v), nil
	case uint16:
		return fmt.Sprintf("%d", v), nil
	case uint32:
		return fmt.Sprintf("%d", v), nil
	case uint64:
		return fmt.Sprintf("%d", v), nil
	case float32:
		return fmt.Sprintf("%g", v), nil
	case float64:
		return fmt.Sprintf("%g", v), nil
	default:
		return "", fmt.Errorf("unsupported value type %T", value)
	}
}

func normalizeJSONNumberValue(value any) any {
	switch v := value.(type) {
	case json.Number:
		if intValue, err := v.Int64(); err == nil {
			return intValue
		}

		if floatValue, err := v.Float64(); err == nil {
			return floatValue
		}

		return string(v)
	case map[string]any:
		for key, child := range v {
			v[key] = normalizeJSONNumberValue(child)
		}

		return v
	case []any:
		for i, child := range v {
			v[i] = normalizeJSONNumberValue(child)
		}

		return v
	default:
		return value
	}
}
