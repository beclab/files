package database

import (
	"fmt"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
)

type Filter struct {
	Field    string
	Operator string
	Value    interface{}
}

type QueryParams struct {
	AND []Filter
	OR  []Filter
}

type JoinCondition struct {
	Table     string
	Field     string
	JoinTable string
	JoinField string
}

func BuildStringQueryParam(reqValue, dbField, op string, params *[]Filter, single bool) {
	if reqValue != "" {
		values := []string{reqValue}
		if !single {
			values = strings.Split(reqValue, ",")
		}
		for _, v := range values {
			*params = append(*params, Filter{
				Field:    dbField,
				Operator: op,
				Value:    v,
			})
		}
	}
}

func BuildIntQueryParam(reqValue interface{}, dbField, op string, params *[]Filter, single bool) {
	if single {
		*params = append(*params, Filter{
			Field:    dbField,
			Operator: op,
			Value:    reqValue.(int),
		})
	} else {
		if reqValue.(string) != "" {
			values := strings.Split(reqValue.(string), ",")
			for _, v := range values {
				vInt, _ := strconv.Atoi(v)
				*params = append(*params, Filter{
					Field:    dbField,
					Operator: op,
					Value:    vInt,
				})
			}
		}
	}
}

func QueryData(
	model interface{},
	result interface{},
	params *QueryParams,
	page, pageSize int64,
	orderBy string,
	order string,
	joinParams []*JoinCondition,
) (int64, error) {
	db := DB.Model(model)

	for _, join := range joinParams {
		db = db.Joins("LEFT JOIN ? ON ? = ?",
			clause.Table{Name: join.JoinTable},
			clause.Column{Table: join.Table, Name: join.Field},
			clause.Column{Table: join.JoinTable, Name: join.JoinField})
	}

	for _, f := range params.AND {
		query, args, err := buildCondition(f.Field, f.Operator, f.Value)
		if err != nil {
			return 0, err
		}
		db = db.Where(query, args...)
	}

	if len(params.OR) > 0 {
		orClauses := make([]string, 0)
		orArgs := make([]interface{}, 0)

		for _, f := range params.OR {
			query, args, err := buildCondition(f.Field, f.Operator, f.Value)
			if err != nil {
				return 0, err
			}
			orClauses = append(orClauses, query)
			orArgs = append(orArgs, args...)
		}

		if len(orClauses) > 0 {
			db = db.Where("("+strings.Join(orClauses, " OR ")+")", orArgs...)
		}
	}

	if order == "" {
		order = "DESC"
	}
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}
	if orderBy != "" {
		db = db.Order(orderBy + " " + order)
	} else {
		db = db.Order("create_time " + order)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return 0, err
	}

	if page > 0 && pageSize > 0 {
		db = db.Limit(int(pageSize)).Offset(int((page - 1) * pageSize))
	}

	return total, db.Find(result).Error
}

func buildCondition(field, op string, value interface{}) (string, []interface{}, error) {
	switch op {
	case "=", ">", "<", ">=", "<=", "!=":
		return fmt.Sprintf("%s %s ?", field, op), []interface{}{value}, nil

	case "LIKE", "ILIKE":
		return fmt.Sprintf("%s LIKE ?", field), []interface{}{fmt.Sprint(value)}, nil // "%" needs to be added manually

	case "IN":
		if slice, ok := value.([]interface{}); ok {
			placeholders := make([]string, len(slice))
			for i := range slice {
				placeholders[i] = "?"
			}
			return fmt.Sprintf("%s IN (%s)", field, strings.Join(placeholders, ",")), slice, nil
		}
		return "", nil, fmt.Errorf("invalid value type for IN operator")

	case "BETWEEN":
		if values, ok := value.([]interface{}); ok && len(values) == 2 {
			return fmt.Sprintf("%s BETWEEN ? AND ?", field), values, nil
		}
		return "", nil, fmt.Errorf("BETWEEN operator needs exactly two values")

	default:
		return "", nil, fmt.Errorf("unsupported operator: %s", op)
	}
}
