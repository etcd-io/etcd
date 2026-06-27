package cat

// On builds a SQL ON clause comparing two columns across tables.
// Formats as: "table1.column1 = table2.column2" with proper spacing.
// Useful in JOIN conditions to match keys between tables.
func On(table1, column1, table2, column2 string) string {
	return With(space,
		With(dot, table1, column1),
		Pad(equal),
		With(dot, table2, column2),
	)
}

// Using builds a SQL condition comparing two aliased columns.
// Formats as: "alias1.column1 = alias2.column2" for JOINs or filters.
// Helps when working with table aliases in complex queries.
func Using(alias1, column1, alias2, column2 string) string {
	return With(space,
		With(dot, alias1, column1),
		Pad(equal),
		With(dot, alias2, column2),
	)
}

// And joins multiple SQL conditions with the AND operator.
// Adds spacing to ensure clean SQL output (e.g., "cond1 AND cond2").
// Accepts variadic arguments for flexible condition chaining.
func And(conditions ...any) string {
	return With(Pad(and), conditions...)
}

// In creates a SQL IN clause with properly quoted values
// Example: In("status", "active", "pending") → "status IN ('active', 'pending')"
// Handles value quoting and comma separation automatically
func In(column string, values ...string) string {
	if len(values) == 0 {
		return Concat(column, inOpen, inClose)
	}

	quotedValues := make([]string, len(values))
	for i, v := range values {
		quotedValues[i] = "'" + v + "'"
	}
	return Concat(column, inOpen, JoinWith(comma+space, quotedValues...), inClose)
}

// As creates an aliased SQL expression
// Example: As("COUNT(*)", "total_count") → "COUNT(*) AS total_count"
func As(expression, alias string) string {
	return Concat(expression, asSQL, alias)
}

// Count creates a COUNT expression with optional alias
// Example: Count("id") → "COUNT(id)"
// Example: Count("id", "total") → "COUNT(id) AS total"
// Example: Count("DISTINCT user_id", "unique_users") → "COUNT(DISTINCT user_id) AS unique_users"
func Count(column string, alias ...string) string {
	expression := Concat(count, column, parenClose)
	if len(alias) == 0 {
		return expression
	}
	return As(expression, alias[0])
}

// CountAll creates COUNT(*) with optional alias
// Example: CountAll() → "COUNT(*)"
// Example: CountAll("total") → "COUNT(*) AS total"
func CountAll(alias ...string) string {
	if len(alias) == 0 {
		return countAll
	}
	return As(countAll, alias[0])
}

// Sum creates a SUM expression with optional alias
// Example: Sum("amount") → "SUM(amount)"
// Example: Sum("amount", "total") → "SUM(amount) AS total"
func Sum(column string, alias ...string) string {
	expression := Concat(sum, column, parenClose)
	if len(alias) == 0 {
		return expression
	}
	return As(expression, alias[0])
}

// Avg creates an AVG expression with optional alias
// Example: Avg("score") → "AVG(score)"
// Example: Avg("score", "average") → "AVG(score) AS average"
func Avg(column string, alias ...string) string {
	expression := Concat(avg, column, parenClose)
	if len(alias) == 0 {
		return expression
	}
	return As(expression, alias[0])
}

// Max creates a MAX expression with optional alias
// Example: Max("price") → "MAX(price)"
// Example: Max("price", "max_price") → "MAX(price) AS max_price"
func Max(column string, alias ...string) string {
	expression := Concat(maxOpen, column, parenClose)
	if len(alias) == 0 {
		return expression
	}
	return As(expression, alias[0])
}

// Min creates a MIN expression with optional alias
// Example: Min("price") → "MIN(price)"
// Example: Min("price", "min_price") → "MIN(price) AS min_price"
func Min(column string, alias ...string) string {
	expression := Concat(minOpen, column, parenClose)
	if len(alias) == 0 {
		return expression
	}
	return As(expression, alias[0])
}

// Case creates a SQL CASE expression with optional alias
// Example: Case("WHEN status = 'active' THEN 1 ELSE 0 END", "is_active") → "CASE WHEN status = 'active' THEN 1 ELSE 0 END AS is_active"
func Case(expression string, alias ...string) string {
	caseExpr := Concat(caseSQL, expression)
	if len(alias) == 0 {
		return caseExpr
	}
	return As(caseExpr, alias[0])
}

// CaseWhen creates a complete SQL CASE expression from individual parts with proper value handling
// Example: CaseWhen("status =", "'active'", "1", "0", "is_active") → "CASE WHEN status = 'active' THEN 1 ELSE 0 END AS is_active"
// Example: CaseWhen("age >", "18", "'adult'", "'minor'", "age_group") → "CASE WHEN age > 18 THEN 'adult' ELSE 'minor' END AS age_group"
func CaseWhen(conditionPart string, conditionValue, thenValue, elseValue any, alias ...string) string {
	condition := Concat(conditionPart, valueToString(conditionValue))
	expression := Concat(
		when, condition, then, valueToString(thenValue), elseSQL, valueToString(elseValue), end,
	)
	return Case(expression, alias...)
}

// CaseWhenMulti creates a SQL CASE expression with multiple WHEN clauses
// Example: CaseWhenMulti([]string{"status =", "age >"}, []any{"'active'", 18}, []any{1, "'adult'"}, 0, "result") → "CASE WHEN status = 'active' THEN 1 WHEN age > 18 THEN 'adult' ELSE 0 END AS result"
func CaseWhenMulti(conditionParts []string, conditionValues, thenValues []any, elseValue any, alias ...string) string {
	if len(conditionParts) != len(conditionValues) || len(conditionParts) != len(thenValues) {
		return "" // or handle error
	}

	var whenClauses []string
	for i := 0; i < len(conditionParts); i++ {
		condition := Concat(conditionParts[i], valueToString(conditionValues[i]))
		whenClause := Concat(when, condition, then, valueToString(thenValues[i]))
		whenClauses = append(whenClauses, whenClause)
	}

	expression := Concat(
		JoinWith(space, whenClauses...),
		elseSQL,
		valueToString(elseValue),
		end,
	)
	return Case(expression, alias...)
}
