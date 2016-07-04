// Copyright 2012, Google Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sqlparser

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/xwb1989/sqlparser/dependency/sqltypes"
)

// Instructions for creating new types: If a type
// needs to satisfy an interface, declare that function
// along with that interface. This will help users
// identify the list of types to which they can assert
// those interfaces.
// If the member of a type has a string with a predefined
// list of values, declare those values as const following
// the type.
// For interfaces that define dummy functions to consolidate
// a set of types, define the function as ITypeName.
// This will help avoid name collisions.

// Parse parses the sql and returns a Statement, which
// is the AST representation of the query.
func Parse(sql string) (Statement, error) {
	tokenizer := NewStringTokenizer(sql)
	if yyParse(tokenizer) != 0 {
		return nil, errors.New(tokenizer.LastError)
	}
	return tokenizer.ParseTree, nil
}

// SQLNode defines the interface for all nodes
// generated by the parser.
type SQLNode interface {
	Format(buf *TrackedBuffer)
}

// String returns a string representation of an SQLNode.
func String(node SQLNode) string {
	buf := NewTrackedBuffer(nil)
	buf.Myprintf("%v", node)
	return buf.String()
}

// Statement represents a statement.
type Statement interface {
	IStatement()
	SQLNode
}

func (*Union) IStatement()  {}
func (*Select) IStatement() {}
func (*Insert) IStatement() {}
func (*Update) IStatement() {}
func (*Delete) IStatement() {}
func (*Set) IStatement()    {}
func (*DDL) IStatement()    {}
func (*Other) IStatement()  {}

// SelectStatement any SELECT statement.
type SelectStatement interface {
	ISelectStatement()
	IStatement()
	IInsertRows()
	SQLNode
}

func (*Select) ISelectStatement() {}
func (*Union) ISelectStatement()  {}

// Select represents a SELECT statement.
type Select struct {
	Comments    Comments
	Distinct    string
	SelectExprs SelectExprs
	From        TableExprs
	Where       *Where
	TimeRange   *TimeRange
	GroupBy     GroupBy
	Having      *Where
	OrderBy     OrderBy
	Limit       *Limit
	Lock        string
}

// Select.Distinct
const (
	AST_DISTINCT = "distinct "
)

// Select.Lock
const (
	AST_FOR_UPDATE = " for update"
	AST_SHARE_MODE = " lock in share mode"
)

func (node *Select) Format(buf *TrackedBuffer) {
	buf.Myprintf("select %v%s%v from %v%v%v%v%v%v%s",
		node.Comments, node.Distinct, node.SelectExprs,
		node.From, node.Where,
		node.GroupBy, node.Having, node.OrderBy,
		node.Limit, node.Lock)
}

// Union represents a UNION statement.
type Union struct {
	Type        string
	Left, Right SelectStatement
}

// Union.Type
const (
	AST_UNION     = "union"
	AST_UNION_ALL = "union all"
	AST_SET_MINUS = "minus"
	AST_EXCEPT    = "except"
	AST_INTERSECT = "intersect"
)

func (node *Union) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v %s %v", node.Left, node.Type, node.Right)
}

// Insert represents an INSERT statement.
type Insert struct {
	Comments Comments
	Table    *TableName
	Columns  Columns
	Rows     InsertRows
	OnDup    OnDup
}

func (node *Insert) Format(buf *TrackedBuffer) {
	buf.Myprintf("insert %vinto %v%v %v%v",
		node.Comments,
		node.Table, node.Columns, node.Rows, node.OnDup)
}

// InsertRows represents the rows for an INSERT statement.
type InsertRows interface {
	IInsertRows()
	SQLNode
}

func (*Select) IInsertRows() {}
func (*Union) IInsertRows()  {}
func (Values) IInsertRows()  {}

// Update represents an UPDATE statement.
type Update struct {
	Comments Comments
	Table    *TableName
	Exprs    UpdateExprs
	Where    *Where
	OrderBy  OrderBy
	Limit    *Limit
}

func (node *Update) Format(buf *TrackedBuffer) {
	buf.Myprintf("update %v%v set %v%v%v%v",
		node.Comments, node.Table,
		node.Exprs, node.Where, node.OrderBy, node.Limit)
}

// Delete represents a DELETE statement.
type Delete struct {
	Comments Comments
	Table    *TableName
	Where    *Where
	OrderBy  OrderBy
	Limit    *Limit
}

func (node *Delete) Format(buf *TrackedBuffer) {
	buf.Myprintf("delete %vfrom %v%v%v%v",
		node.Comments,
		node.Table, node.Where, node.OrderBy, node.Limit)
}

// Set represents a SET statement.
type Set struct {
	Comments Comments
	Exprs    UpdateExprs
}

func (node *Set) Format(buf *TrackedBuffer) {
	buf.Myprintf("set %v%v", node.Comments, node.Exprs)
}

// DDL represents a CREATE, ALTER, DROP or RENAME statement.
// Table is set for AST_ALTER, AST_DROP, AST_RENAME.
// NewName is set for AST_ALTER, AST_CREATE, AST_RENAME.
type DDL struct {
	Action  string
	Table   []byte
	NewName []byte
}

type ColumnAtts []string

func (node ColumnAtts) Format(buf *TrackedBuffer) {
	prefix := " "
	for _, v := range node {
		if v != "" {
			buf.Myprintf("%s%s", prefix, v)
		}
	}
}

type ColumnDefinition struct {
	ColName    string
	ColType    string
	ColumnAtts ColumnAtts
}

func (node ColumnDefinition) Format(buf *TrackedBuffer) {
	buf.Myprintf("%s %s%v", node.ColName, node.ColType, node.ColumnAtts)
}

type ColumnDefinitions []*ColumnDefinition

func (node ColumnDefinitions) Format(buf *TrackedBuffer) {
	prefix := ""
	buf.Myprintf("(\n")
	for i := 0; i < len(node); i++ {
		buf.Myprintf("%s\t%v", prefix, node[i])
		prefix = ",\n"
	}
	buf.Myprintf("\n)")
}

type CreateTable struct {
	Name              []byte
	ColumnDefinitions ColumnDefinitions
}

func (node *CreateTable) Format(buf *TrackedBuffer) {
	buf.Myprintf("create table %s %v", node.Name, node.ColumnDefinitions)
}
func (node *CreateTable) IStatement() {}

const (
	AST_TABLE = "table"
	AST_VIEW  = "view"
)

const (
	AST_CREATE = "create"
	AST_ALTER  = "alter"
	AST_DROP   = "drop"
	AST_RENAME = "rename"
)

func (node *DDL) Format(buf *TrackedBuffer) {
	switch node.Action {
	case AST_CREATE:
		buf.Myprintf("%s table %s", node.Action, node.NewName)
	case AST_RENAME:
		buf.Myprintf("%s table %s %s", node.Action, node.Table, node.NewName)
	default:
		buf.Myprintf("%s table %s", node.Action, node.Table)
	}
}

// Other represents a SHOW, DESCRIBE, or EXPLAIN statement.
// It should be used only as an indicator. It does not contain
// the full AST for the statement.
type Other struct{}

func (node *Other) Format(buf *TrackedBuffer) {
	buf.WriteString("other")
}

// Comments represents a list of comments.
type Comments [][]byte

func (node Comments) Format(buf *TrackedBuffer) {
	for _, c := range node {
		buf.Myprintf("%s ", c)
	}
}

// SelectExprs represents SELECT expressions.
type SelectExprs []SelectExpr

func (node SelectExprs) Format(buf *TrackedBuffer) {
	var prefix string
	for _, n := range node {
		buf.Myprintf("%s%v", prefix, n)
		prefix = ", "
	}
}

// SelectExpr represents a SELECT expression.
type SelectExpr interface {
	ISelectExpr()
	SQLNode
}

func (*StarExpr) ISelectExpr()    {}
func (*NonStarExpr) ISelectExpr() {}

// StarExpr defines a '*' or 'table.*' expression.
type StarExpr struct {
	TableName []byte
}

func (node *StarExpr) Format(buf *TrackedBuffer) {
	if node.TableName != nil {
		buf.Myprintf("%s.", node.TableName)
	}
	buf.Myprintf("*")
}

// NonStarExpr defines a non-'*' select expr.
type NonStarExpr struct {
	Expr Expr
	As   []byte
}

func (node *NonStarExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v", node.Expr)
	if node.As != nil {
		buf.Myprintf(" as %s", node.As)
	}
}

// Columns represents an insert column list.
// The syntax for Columns is a subset of SelectExprs.
// So, it's castable to a SelectExprs and can be analyzed
// as such.
type Columns []SelectExpr

func (node Columns) Format(buf *TrackedBuffer) {
	if node == nil {
		return
	}
	buf.Myprintf("(%v)", SelectExprs(node))
}

// TableExprs represents a list of table expressions.
type TableExprs []TableExpr

func (node TableExprs) Format(buf *TrackedBuffer) {
	var prefix string
	for _, n := range node {
		buf.Myprintf("%s%v", prefix, n)
		prefix = ", "
	}
}

// TableExpr represents a table expression.
type TableExpr interface {
	ITableExpr()
	SQLNode
}

func (*AliasedTableExpr) ITableExpr() {}
func (*ParenTableExpr) ITableExpr()   {}
func (*JoinTableExpr) ITableExpr()    {}

// AliasedTableExpr represents a table expression
// coupled with an optional alias or index hint.
type AliasedTableExpr struct {
	Expr  SimpleTableExpr
	As    []byte
	Hints *IndexHints
}

func (node *AliasedTableExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v", node.Expr)
	if node.As != nil {
		buf.Myprintf(" as %s", node.As)
	}
	if node.Hints != nil {
		// Hint node provides the space padding.
		buf.Myprintf("%v", node.Hints)
	}
}

// SimpleTableExpr represents a simple table expression.
type SimpleTableExpr interface {
	ISimpleTableExpr()
	SQLNode
}

func (*TableName) ISimpleTableExpr() {}
func (*Subquery) ISimpleTableExpr()  {}

// TableName represents a table  name.
type TableName struct {
	Name, Qualifier []byte
}

func (node *TableName) Format(buf *TrackedBuffer) {
	if node.Qualifier != nil {
		escape(buf, node.Qualifier)
		buf.Myprintf(".")
	}
	escape(buf, node.Name)
}

// ParenTableExpr represents a parenthesized TableExpr.
type ParenTableExpr struct {
	Expr TableExpr
}

func (node *ParenTableExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("(%v)", node.Expr)
}

// JoinTableExpr represents a TableExpr that's a JOIN operation.
type JoinTableExpr struct {
	LeftExpr  TableExpr
	Join      string
	RightExpr TableExpr
	On        BoolExpr
}

// JoinTableExpr.Join
const (
	AST_JOIN          = "join"
	AST_STRAIGHT_JOIN = "straight_join"
	AST_LEFT_JOIN     = "left join"
	AST_RIGHT_JOIN    = "right join"
	AST_CROSS_JOIN    = "cross join"
	AST_NATURAL_JOIN  = "natural join"
)

func (node *JoinTableExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v %s %v", node.LeftExpr, node.Join, node.RightExpr)
	if node.On != nil {
		buf.Myprintf(" on %v", node.On)
	}
}

// IndexHints represents a list of index hints.
type IndexHints struct {
	Type    string
	Indexes [][]byte
}

const (
	AST_USE    = "use"
	AST_IGNORE = "ignore"
	AST_FORCE  = "force"
)

func (node *IndexHints) Format(buf *TrackedBuffer) {
	buf.Myprintf(" %s index ", node.Type)
	prefix := "("
	for _, n := range node.Indexes {
		buf.Myprintf("%s%s", prefix, n)
		prefix = ", "
	}
	buf.Myprintf(")")
}

// Where represents a WHERE or HAVING clause.
type Where struct {
	Type string
	Expr BoolExpr
}

// Where.Type
const (
	AST_WHERE  = "where"
	AST_HAVING = "having"
)

// NewWhere creates a WHERE or HAVING clause out
// of a BoolExpr. If the expression is nil, it returns nil.
func NewWhere(typ string, expr BoolExpr) *Where {
	if expr == nil {
		return nil
	}
	return &Where{Type: typ, Expr: expr}
}

func (node *Where) Format(buf *TrackedBuffer) {
	if node == nil {
		return
	}
	buf.Myprintf(" %s %v", node.Type, node.Expr)
}

// TimeRange represents a TIMERANGE clause.
type TimeRange struct {
	From, To string
}

func (node *TimeRange) Format(buf *TrackedBuffer) {
	if node == nil {
		return
	}
	buf.Myprintf(" timerange ")
	buf.Myprintf("%v", string(node.From))
	if node.To != "" {
		buf.Myprintf(", %v", string(node.To))
	}
}

// Expr represents an expression.
type Expr interface {
	IExpr()
	SQLNode
}

func (*AndExpr) IExpr()        {}
func (*OrExpr) IExpr()         {}
func (*NotExpr) IExpr()        {}
func (*ParenBoolExpr) IExpr()  {}
func (*ComparisonExpr) IExpr() {}
func (*RangeCond) IExpr()      {}
func (*NullCheck) IExpr()      {}
func (*ExistsExpr) IExpr()     {}
func (StrVal) IExpr()          {}
func (NumVal) IExpr()          {}
func (ValArg) IExpr()          {}
func (*NullVal) IExpr()        {}
func (*ColName) IExpr()        {}
func (ValTuple) IExpr()        {}
func (*Subquery) IExpr()       {}
func (ListArg) IExpr()         {}
func (*BinaryExpr) IExpr()     {}
func (*UnaryExpr) IExpr()      {}
func (*FuncExpr) IExpr()       {}
func (*CaseExpr) IExpr()       {}

// BoolExpr represents a boolean expression.
type BoolExpr interface {
	IBoolExpr()
	Expr
}

func (*AndExpr) IBoolExpr()        {}
func (*OrExpr) IBoolExpr()         {}
func (*NotExpr) IBoolExpr()        {}
func (*ParenBoolExpr) IBoolExpr()  {}
func (*ComparisonExpr) IBoolExpr() {}
func (*RangeCond) IBoolExpr()      {}
func (*NullCheck) IBoolExpr()      {}
func (*ExistsExpr) IBoolExpr()     {}

// AndExpr represents an AND expression.
type AndExpr struct {
	Left, Right BoolExpr
}

func (node *AndExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v and %v", node.Left, node.Right)
}

// OrExpr represents an OR expression.
type OrExpr struct {
	Left, Right BoolExpr
}

func (node *OrExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v or %v", node.Left, node.Right)
}

// NotExpr represents a NOT expression.
type NotExpr struct {
	Expr BoolExpr
}

func (node *NotExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("not %v", node.Expr)
}

// ParenBoolExpr represents a parenthesized boolean expression.
type ParenBoolExpr struct {
	Expr BoolExpr
}

func (node *ParenBoolExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("(%v)", node.Expr)
}

// ComparisonExpr represents a two-value comparison expression.
type ComparisonExpr struct {
	Operator    string
	Left, Right ValExpr
}

// ComparisonExpr.Operator
const (
	AST_EQ       = "="
	AST_LT       = "<"
	AST_GT       = ">"
	AST_LE       = "<="
	AST_GE       = ">="
	AST_NE       = "!="
	AST_NSE      = "<=>"
	AST_IN       = "in"
	AST_NOT_IN   = "not in"
	AST_LIKE     = "like"
	AST_NOT_LIKE = "not like"
)

func (node *ComparisonExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v %s %v", node.Left, node.Operator, node.Right)
}

// RangeCond represents a BETWEEN or a NOT BETWEEN expression.
type RangeCond struct {
	Operator string
	Left     ValExpr
	From, To ValExpr
}

// RangeCond.Operator
const (
	AST_BETWEEN     = "between"
	AST_NOT_BETWEEN = "not between"
)

func (node *RangeCond) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v %s %v and %v", node.Left, node.Operator, node.From, node.To)
}

// NullCheck represents an IS NULL or an IS NOT NULL expression.
type NullCheck struct {
	Operator string
	Expr     ValExpr
}

// NullCheck.Operator
const (
	AST_IS_NULL     = "is null"
	AST_IS_NOT_NULL = "is not null"
)

func (node *NullCheck) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v %s", node.Expr, node.Operator)
}

// ExistsExpr represents an EXISTS expression.
type ExistsExpr struct {
	Subquery *Subquery
}

func (node *ExistsExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("exists %v", node.Subquery)
}

// ValExpr represents a value expression.
type ValExpr interface {
	IValExpr()
	Expr
}

func (StrVal) IValExpr()      {}
func (NumVal) IValExpr()      {}
func (ValArg) IValExpr()      {}
func (*NullVal) IValExpr()    {}
func (*ColName) IValExpr()    {}
func (ValTuple) IValExpr()    {}
func (*Subquery) IValExpr()   {}
func (ListArg) IValExpr()     {}
func (*BinaryExpr) IValExpr() {}
func (*UnaryExpr) IValExpr()  {}
func (*FuncExpr) IValExpr()   {}
func (*CaseExpr) IValExpr()   {}

// StrVal represents a string value.
type StrVal []byte

func (node StrVal) Format(buf *TrackedBuffer) {
	s := sqltypes.MakeString([]byte(node))
	s.EncodeSql(buf)
}

// NumVal represents a number.
type NumVal []byte

func (node NumVal) Format(buf *TrackedBuffer) {
	buf.Myprintf("%s", []byte(node))
}

// ValArg represents a named bind var argument.
type ValArg []byte

func (node ValArg) Format(buf *TrackedBuffer) {
	buf.WriteArg(string(node))
}

// NullVal represents a NULL value.
type NullVal struct{}

func (node *NullVal) Format(buf *TrackedBuffer) {
	buf.Myprintf("null")
}

// ColName represents a column name.
type ColName struct {
	Name, Qualifier []byte
}

func (node *ColName) Format(buf *TrackedBuffer) {
	if node.Qualifier != nil {
		escape(buf, node.Qualifier)
		buf.Myprintf(".")
	}
	escape(buf, node.Name)
}

func escape(buf *TrackedBuffer, name []byte) {
	if _, ok := keywords[string(name)]; ok {
		buf.Myprintf("`%s`", name)
	} else {
		buf.Myprintf("%s", name)
	}
}

// ColTuple represents a list of column values.
// It can be ValTuple, Subquery, ListArg.
type ColTuple interface {
	IColTuple()
	ValExpr
}

func (ValTuple) IColTuple()  {}
func (*Subquery) IColTuple() {}
func (ListArg) IColTuple()   {}

// ValTuple represents a tuple of actual values.
type ValTuple ValExprs

func (node ValTuple) Format(buf *TrackedBuffer) {
	buf.Myprintf("(%v)", ValExprs(node))
}

// ValExprs represents a list of value expressions.
// It's not a valid expression because it's not parenthesized.
type ValExprs []ValExpr

func (node ValExprs) Format(buf *TrackedBuffer) {
	var prefix string
	for _, n := range node {
		buf.Myprintf("%s%v", prefix, n)
		prefix = ", "
	}
}

// Subquery represents a subquery.
type Subquery struct {
	Select SelectStatement
}

func (node *Subquery) Format(buf *TrackedBuffer) {
	buf.Myprintf("(%v)", node.Select)
}

// ListArg represents a named list argument.
type ListArg []byte

func (node ListArg) Format(buf *TrackedBuffer) {
	buf.WriteArg(string(node))
}

// BinaryExpr represents a binary value expression.
type BinaryExpr struct {
	Operator    byte
	Left, Right Expr
}

// BinaryExpr.Operator
const (
	AST_BITAND = '&'
	AST_BITOR  = '|'
	AST_BITXOR = '^'
	AST_PLUS   = '+'
	AST_MINUS  = '-'
	AST_MULT   = '*'
	AST_DIV    = '/'
	AST_MOD    = '%'
)

func (node *BinaryExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v%c%v", node.Left, node.Operator, node.Right)
}

// UnaryExpr represents a unary value expression.
type UnaryExpr struct {
	Operator byte
	Expr     Expr
}

// UnaryExpr.Operator
const (
	AST_UPLUS  = '+'
	AST_UMINUS = '-'
	AST_TILDA  = '~'
)

func (node *UnaryExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%c%v", node.Operator, node.Expr)
}

// FuncExpr represents a function call.
type FuncExpr struct {
	Name     []byte
	Distinct bool
	Exprs    SelectExprs
}

func (node *FuncExpr) Format(buf *TrackedBuffer) {
	var distinct string
	if node.Distinct {
		distinct = "distinct "
	}
	buf.Myprintf("%s(%s%v)", node.Name, distinct, node.Exprs)
}

// Aggregates is a map of all aggregate functions.
var Aggregates = map[string]bool{
	"avg":          true,
	"bit_and":      true,
	"bit_or":       true,
	"bit_xor":      true,
	"count":        true,
	"group_concat": true,
	"max":          true,
	"min":          true,
	"std":          true,
	"stddev_pop":   true,
	"stddev_samp":  true,
	"stddev":       true,
	"sum":          true,
	"var_pop":      true,
	"var_samp":     true,
	"variance":     true,
}

func (node *FuncExpr) IsAggregate() bool {
	return Aggregates[string(node.Name)]
}

// CaseExpr represents a CASE expression.
type CaseExpr struct {
	Expr  ValExpr
	Whens []*When
	Else  ValExpr
}

func (node *CaseExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("case ")
	if node.Expr != nil {
		buf.Myprintf("%v ", node.Expr)
	}
	for _, when := range node.Whens {
		buf.Myprintf("%v ", when)
	}
	if node.Else != nil {
		buf.Myprintf("else %v ", node.Else)
	}
	buf.Myprintf("end")
}

// When represents a WHEN sub-expression.
type When struct {
	Cond BoolExpr
	Val  ValExpr
}

func (node *When) Format(buf *TrackedBuffer) {
	buf.Myprintf("when %v then %v", node.Cond, node.Val)
}

// GroupBy represents a GROUP BY clause.
type GroupBy []ValExpr

func (node GroupBy) Format(buf *TrackedBuffer) {
	prefix := " group by "
	for _, n := range node {
		buf.Myprintf("%s%v", prefix, n)
		prefix = ", "
	}
}

// OrderBy represents an ORDER By clause.
type OrderBy []*Order

func (node OrderBy) Format(buf *TrackedBuffer) {
	prefix := " order by "
	for _, n := range node {
		buf.Myprintf("%s%v", prefix, n)
		prefix = ", "
	}
}

// Order represents an ordering expression.
type Order struct {
	Expr      ValExpr
	Direction string
}

// Order.Direction
const (
	AST_ASC  = "asc"
	AST_DESC = "desc"
)

func (node *Order) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v %s", node.Expr, node.Direction)
}

// Limit represents a LIMIT clause.
type Limit struct {
	Offset, Rowcount ValExpr
}

func (node *Limit) Format(buf *TrackedBuffer) {
	if node == nil {
		return
	}
	buf.Myprintf(" limit ")
	if node.Offset != nil {
		buf.Myprintf("%v, ", node.Offset)
	}
	buf.Myprintf("%v", node.Rowcount)
}

// Limits returns the values of the LIMIT clause as interfaces.
// The returned values can be nil for absent field, string for
// bind variable names, or int64 for an actual number.
// Otherwise, it's an error.
func (node *Limit) Limits() (offset, rowcount interface{}, err error) {
	if node == nil {
		return nil, nil, nil
	}
	switch v := node.Offset.(type) {
	case NumVal:
		o, err := strconv.ParseInt(string(v), 0, 64)
		if err != nil {
			return nil, nil, err
		}
		if o < 0 {
			return nil, nil, fmt.Errorf("negative offset: %d", o)
		}
		offset = o
	case ValArg:
		offset = string(v)
	case nil:
		// pass
	default:
		return nil, nil, fmt.Errorf("unexpected node for offset: %+v", v)
	}
	switch v := node.Rowcount.(type) {
	case NumVal:
		rc, err := strconv.ParseInt(string(v), 0, 64)
		if err != nil {
			return nil, nil, err
		}
		if rc < 0 {
			return nil, nil, fmt.Errorf("negative limit: %d", rc)
		}
		rowcount = rc
	case ValArg:
		rowcount = string(v)
	default:
		return nil, nil, fmt.Errorf("unexpected node for rowcount: %+v", v)
	}
	return offset, rowcount, nil
}

// Values represents a VALUES clause.
type Values []RowTuple

func (node Values) Format(buf *TrackedBuffer) {
	prefix := "values "
	for _, n := range node {
		buf.Myprintf("%s%v", prefix, n)
		prefix = ", "
	}
}

// RowTuple represents a row of values. It can be ValTuple, Subquery.
type RowTuple interface {
	IRowTuple()
	ValExpr
}

func (ValTuple) IRowTuple()  {}
func (*Subquery) IRowTuple() {}

// UpdateExprs represents a list of update expressions.
type UpdateExprs []*UpdateExpr

func (node UpdateExprs) Format(buf *TrackedBuffer) {
	var prefix string
	for _, n := range node {
		buf.Myprintf("%s%v", prefix, n)
		prefix = ", "
	}
}

// UpdateExpr represents an update expression.
type UpdateExpr struct {
	Name *ColName
	Expr ValExpr
}

func (node *UpdateExpr) Format(buf *TrackedBuffer) {
	buf.Myprintf("%v = %v", node.Name, node.Expr)
}

// OnDup represents an ON DUPLICATE KEY clause.
type OnDup UpdateExprs

func (node OnDup) Format(buf *TrackedBuffer) {
	if node == nil {
		return
	}
	buf.Myprintf(" on duplicate key update %v", UpdateExprs(node))
}

//ast keywords added for create table parsing

const (
	//other keywords
	AST_UNSIGNED = "unsigned"
	AST_ZEROFILL = "zerofill"

	//datatypes
	AST_BIT       = "bit"
	AST_TINYINT   = "tinyint"
	AST_SMALLINT  = "smallint"
	AST_MEDIUMINT = "mediumint"
	AST_INT       = "int"
	AST_INTEGER   = "integer"
	AST_BIGINT    = "bigint"

	AST_REAL    = "real"
	AST_DOUBLE  = "double"
	AST_FLOAT   = "float"
	AST_DECIMAL = "decimal"
	AST_NUMERIC = "numeric"

	AST_CHAR    = "char"
	AST_VARCHAR = "varchar"
	AST_TEXT    = "text"

	AST_DATE      = "date"
	AST_TIME      = "time"
	AST_TIMESTAMP = "timestamp"
	AST_DATETIME  = "datetime"
	AST_YEAR      = "year"

	AST_PRIMARY_KEY = "primary key"

	AST_UNIQUE_KEY     = "unique key"
	AST_AUTO_INCREMENT = "auto_increment"
	AST_NOT_NULL       = "not null"
	AST_DEFAULT        = "default"
	AST_KEY            = "key"
)
