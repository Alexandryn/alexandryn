package postgres

import "strconv"

// appendOwnerScope adds one owner-column predicate to a WHERE clause under
// construction. A non-empty value becomes a direct, index-usable
// "col = $N" comparison; an empty value becomes "col IS NULL" and adds no
// argument. This replaces the earlier COALESCE(col, '') = COALESCE($, '')
// form, which was not sargable and treated a NULL owner as a match for an
// empty scope (#118). Reading handlers reject an empty user or library
// before they reach a scoped repository method; the IS NULL branch exists
// only for the pre-tenancy bare Find/Save methods still used by tests.
func appendOwnerScope(clauses []string, args []any, col, value string) ([]string, []any) {
	if value == "" {
		return append(clauses, col+" IS NULL"), args
	}
	args = append(args, value)
	return append(clauses, col+" = $"+strconv.Itoa(len(args))), args
}
