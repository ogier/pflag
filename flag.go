// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package flag implements command-line flag parsing.

	Usage:

	Define flags using flag.String(), Bool(), Int(), etc. Example:
		import "flag"
		var ip *int = flag.Int("flagname", 1234, "help message for flagname")
	If you like, you can bind the flag to a variable using the Var() functions.
		var flagvar int
		func init() {
			flag.IntVar(&flagvar, "flagname", 1234, "help message for flagname")
		}
	Or you can create custom flags that satisfy the Value interface (with
	pointer receivers) and couple them to flag parsing by
		flag.Var(&flagVal, "name", "help message for flagname")
	For such flags, the default value is just the initial value of the variable.

	After all flags are defined, call
		flag.Parse()
	to parse the command line into the defined flags.

	Flags may then be used directly. If you're using the flags themselves,
	they are all pointers; if you bind to variables, they're values.
		fmt.Println("ip has value ", *ip);
		fmt.Println("flagvar has value ", flagvar);

	After parsing, the arguments after the flag are available as the
	slice flag.Args() or individually as flag.Arg(i).
	The arguments are indexed from 0 up to flag.NArg().

	Command line flag syntax:
		-flag
		-flag=x
		-flag x  // non-boolean flags only
	One or two minus signs may be used; they are equivalent.
	The last form is not permitted for boolean flags because the
	meaning of the command
		cmd -x *
	will change if there is a file called 0, false, etc.  You must
	use the -flag=false form to turn off a boolean flag.

	Flag parsing stops just before the first non-flag argument
	("-" is a non-flag argument) or after the terminator "--".

	Integer flags accept 1234, 0664, 0x1234 and may be negative.
	Boolean flags may be 1, 0, t, f, true, false, TRUE, FALSE, True, False.
	Duration flags accept any input valid for time.ParseDuration.

	The default set of command-line flags is controlled by
	top-level functions.  The FlagSet type allows one to define
	independent sets of flags, such as to implement subcommands
	in a command-line interface. The methods of FlagSet are
	analogous to the top-level functions for the command-line
	flag set.
*/
package flag

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"
)

// ErrHelp is the error returned if the flag -help is invoked but no such flag is defined.
var ErrHelp = errors.New("flag: help requested")

// -- bool Value
type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	*b = boolValue(v)
	return err
}

func (b *boolValue) String() string { return fmt.Sprintf("%v", *b) }

// -- int Value
type intValue int

func newIntValue(val int, p *int) *intValue {
	*p = val
	return (*intValue)(p)
}

func (i *intValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	*i = intValue(v)
	return err
}

func (i *intValue) String() string { return fmt.Sprintf("%v", *i) }

// -- int64 Value
type int64Value int64

func newInt64Value(val int64, p *int64) *int64Value {
	*p = val
	return (*int64Value)(p)
}

func (i *int64Value) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	*i = int64Value(v)
	return err
}

func (i *int64Value) String() string { return fmt.Sprintf("%v", *i) }

// -- uint Value
type uintValue uint

func newUintValue(val uint, p *uint) *uintValue {
	*p = val
	return (*uintValue)(p)
}

func (i *uintValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	*i = uintValue(v)
	return err
}

func (i *uintValue) String() string { return fmt.Sprintf("%v", *i) }

// -- uint64 Value
type uint64Value uint64

func newUint64Value(val uint64, p *uint64) *uint64Value {
	*p = val
	return (*uint64Value)(p)
}

func (i *uint64Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	*i = uint64Value(v)
	return err
}

func (i *uint64Value) String() string { return fmt.Sprintf("%v", *i) }

// -- string Value
type stringValue string

func newStringValue(val string, p *string) *stringValue {
	*p = val
	return (*stringValue)(p)
}

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

func (s *stringValue) String() string { return fmt.Sprintf("%s", *s) }

// -- float64 Value
type float64Value float64

func newFloat64Value(val float64, p *float64) *float64Value {
	*p = val
	return (*float64Value)(p)
}

func (f *float64Value) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	*f = float64Value(v)
	return err
}

func (f *float64Value) String() string { return fmt.Sprintf("%v", *f) }

// -- time.Duration Value
type durationValue time.Duration

func newDurationValue(val time.Duration, p *time.Duration) *durationValue {
	*p = val
	return (*durationValue)(p)
}

func (d *durationValue) Set(s string) error {
	v, err := time.ParseDuration(s)
	*d = durationValue(v)
	return err
}

func (d *durationValue) String() string { return (*time.Duration)(d).String() }

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
type Value interface {
	String() string
	Set(string) error
}

// ErrorHandling defines how to handle flag parsing errors.
type ErrorHandling int

const (
	ContinueOnError ErrorHandling = iota
	ExitOnError
	PanicOnError
)

// A FlagSet represents a set of defined flags.
type FlagSet struct {
	// Usage is the function called when an error occurs while parsing flags.
	// The field is a function (not a method) that may be changed to point to
	// a custom error handler.
	Usage func()

	name          string
	parsed        bool
	actual        map[string]*Flag
	formal        map[string]*Flag
	shortcuts     map[byte]*Flag
	args          []string // arguments after flags
	exitOnError   bool     // does the program exit if there's an error?
	errorHandling ErrorHandling
	output        io.Writer // nil means stderr; use out() accessor
}

// A Flag represents the state of a flag.
type Flag struct {
	Name     string // name as it appears on command line
	Shortcut string // one-letter abbreviated flag
	Usage    string // help message
	Value    Value  // value as set
	DefValue string // default value (as text); for usage message
}

// sortFlags returns the flags as a slice in lexicographical sorted order.
func sortFlags(flags map[string]*Flag) []*Flag {
	list := make(sort.StringSlice, len(flags))
	i := 0
	for _, f := range flags {
		list[i] = f.Name
		i++
	}
	list.Sort()
	result := make([]*Flag, len(list))
	for i, name := range list {
		result[i] = flags[name]
	}
	return result
}

func (f *FlagSet) out() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
func (f *FlagSet) SetOutput(output io.Writer) {
	f.output = output
}

// VisitAll visits the flags in lexicographical order, calling fn for each.
// It visits all flags, even those not set.
func (f *FlagSet) VisitAll(fn func(*Flag)) {
	for _, flag := range sortFlags(f.formal) {
		fn(flag)
	}
}

// VisitAll visits the command-line flags in lexicographical order, calling
// fn for each.  It visits all flags, even those not set.
func VisitAll(fn func(*Flag)) {
	commandLine.VisitAll(fn)
}

// Visit visits the flags in lexicographical order, calling fn for each.
// It visits only those flags that have been set.
func (f *FlagSet) Visit(fn func(*Flag)) {
	for _, flag := range sortFlags(f.actual) {
		fn(flag)
	}
}

// Visit visits the command-line flags in lexicographical order, calling fn
// for each.  It visits only those flags that have been set.
func Visit(fn func(*Flag)) {
	commandLine.Visit(fn)
}

// Lookup returns the Flag structure of the named flag, returning nil if none exists.
func (f *FlagSet) Lookup(name string) *Flag {
	return f.formal[name]
}

// Lookup returns the Flag structure of the named command-line flag,
// returning nil if none exists.
func Lookup(name string) *Flag {
	return commandLine.formal[name]
}

// Set sets the value of the named flag.
func (f *FlagSet) Set(name, value string) error {
	flag, ok := f.formal[name]
	if !ok {
		return fmt.Errorf("no such flag -%v", name)
	}
	err := flag.Value.Set(value)
	if err != nil {
		return err
	}
	if f.actual == nil {
		f.actual = make(map[string]*Flag)
	}
	f.actual[name] = flag
	return nil
}

// Set sets the value of the named command-line flag.
func Set(name, value string) error {
	return commandLine.Set(name, value)
}

// PrintDefaults prints, to standard error unless configured
// otherwise, the default values of all defined flags in the set.
func (f *FlagSet) PrintDefaults() {
	f.VisitAll(func(flag *Flag) {
		format := "  --%s=%s: %s\n"
		if _, ok := flag.Value.(*stringValue); ok {
			// put quotes on the value
			format = "  --%s=%q: %s\n"
		}
		if len(flag.Shortcut) > 0 {
			format = "  -%s," + format[1:]
		} else {
			format = "%s" + format
		}
		fmt.Fprintf(f.out(), format, flag.Shortcut, flag.Name, flag.DefValue, flag.Usage)
	})
}

// PrintDefaults prints to standard error the default values of all defined command-line flags.
func PrintDefaults() {
	commandLine.PrintDefaults()
}

// defaultUsage is the default function to print a usage message.
func defaultUsage(f *FlagSet) {
	fmt.Fprintf(f.out(), "Usage of %s:\n", f.name)
	f.PrintDefaults()
}

// NOTE: Usage is not just defaultUsage(commandLine)
// because it serves (via godoc flag Usage) as the example
// for how to write your own usage function.

// Usage prints to standard error a usage message documenting all defined command-line flags.
// The function is a variable that may be changed to point to a custom function.
var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	PrintDefaults()
}

// NFlag returns the number of flags that have been set.
func (f *FlagSet) NFlag() int { return len(f.actual) }

// NFlag returns the number of command-line flags that have been set.
func NFlag() int { return len(commandLine.actual) }

// Arg returns the i'th argument.  Arg(0) is the first remaining argument
// after flags have been processed.
func (f *FlagSet) Arg(i int) string {
	if i < 0 || i >= len(f.args) {
		return ""
	}
	return f.args[i]
}

// Arg returns the i'th command-line argument.  Arg(0) is the first remaining argument
// after flags have been processed.
func Arg(i int) string {
	return commandLine.Arg(i)
}

// NArg is the number of arguments remaining after flags have been processed.
func (f *FlagSet) NArg() int { return len(f.args) }

// NArg is the number of arguments remaining after flags have been processed.
func NArg() int { return len(commandLine.args) }

// Args returns the non-flag arguments.
func (f *FlagSet) Args() []string { return f.args }

// Args returns the non-flag command-line arguments.
func Args() []string { return commandLine.args }

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.
func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	f.VarP(newBoolValue(value, p), name, "", usage)
}

// Like BoolVar, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) BoolVarP(p *bool, name, shortcut string, value bool, usage string) {
	f.VarP(newBoolValue(value, p), name, shortcut, usage)
}

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.
func BoolVar(p *bool, name string, value bool, usage string) {
	commandLine.VarP(newBoolValue(value, p), name, "", usage)
}

// Like BoolVar, but accepts a shortcut letter that can be used after a single dash.
func BoolVarP(p *bool, name, shortcut string, value bool, usage string) {
	commandLine.VarP(newBoolValue(value, p), name, shortcut, usage)
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.
func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVarP(p, name, "", value, usage)
	return p
}

// Like Bool, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) BoolP(name, shortcut string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVarP(p, name, shortcut, value, usage)
	return p
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.
func Bool(name string, value bool, usage string) *bool {
	return commandLine.BoolP(name, "", value, usage)
}

// Like Bool, but accepts a shortcut letter that can be used after a single dash.
func BoolP(name, shortcut string, value bool, usage string) *bool {
	return commandLine.BoolP(name, shortcut, value, usage)
}

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.
func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	f.VarP(newIntValue(value, p), name, "", usage)
}

// Like IntVar, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) IntVarP(p *int, name, shortcut string, value int, usage string) {
	f.VarP(newIntValue(value, p), name, shortcut, usage)
}

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.
func IntVar(p *int, name string, value int, usage string) {
	commandLine.VarP(newIntValue(value, p), name, "", usage)
}

// Like IntVar, but accepts a shortcut letter that can be used after a single dash.
func IntVarP(p *int, name, shortcut string, value int, usage string) {
	commandLine.VarP(newIntValue(value, p), name, shortcut, usage)
}

// Int defines an int flag with specified name, default value, and usage string.
// The return value is the address of an int variable that stores the value of the flag.
func (f *FlagSet) Int(name string, value int, usage string) *int {
	p := new(int)
	f.IntVarP(p, name, "", value, usage)
	return p
}

// Like Int, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) IntP(name, shortcut string, value int, usage string) *int {
	p := new(int)
	f.IntVarP(p, name, shortcut, value, usage)
	return p
}

// Int defines an int flag with specified name, default value, and usage string.
// The return value is the address of an int variable that stores the value of the flag.
func Int(name string, value int, usage string) *int {
	return commandLine.IntP(name, "", value, usage)
}

// Like Int, but accepts a shortcut letter that can be used after a single dash.
func IntP(name, shortcut string, value int, usage string) *int {
	return commandLine.IntP(name, shortcut, value, usage)
}

// Int64Var defines an int64 flag with specified name, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the flag.
func (f *FlagSet) Int64Var(p *int64, name string, value int64, usage string) {
	f.VarP(newInt64Value(value, p), name, "", usage)
}

// Like Int64Var, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) Int64VarP(p *int64, name, shortcut string, value int64, usage string) {
	f.VarP(newInt64Value(value, p), name, shortcut, usage)
}

// Int64Var defines an int64 flag with specified name, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the flag.
func Int64Var(p *int64, name string, value int64, usage string) {
	commandLine.VarP(newInt64Value(value, p), name, "", usage)
}

// Like Int64Var, but accepts a shortcut letter that can be used after a single dash.
func Int64VarP(p *int64, name, shortcut string, value int64, usage string) {
	commandLine.VarP(newInt64Value(value, p), name, shortcut, usage)
}

// Int64 defines an int64 flag with specified name, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the flag.
func (f *FlagSet) Int64(name string, value int64, usage string) *int64 {
	p := new(int64)
	f.Int64VarP(p, name, "", value, usage)
	return p
}

// Like Int64, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) Int64P(name, shortcut string, value int64, usage string) *int64 {
	p := new(int64)
	f.Int64VarP(p, name, shortcut, value, usage)
	return p
}

// Int64 defines an int64 flag with specified name, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the flag.
func Int64(name string, value int64, usage string) *int64 {
	return commandLine.Int64P(name, "", value, usage)
}

// Like Int64, but accepts a shortcut letter that can be used after a single dash.
func Int64P(name, shortcut string, value int64, usage string) *int64 {
	return commandLine.Int64P(name, shortcut, value, usage)
}

// UintVar defines a uint flag with specified name, default value, and usage string.
// The argument p points to a uint variable in which to store the value of the flag.
func (f *FlagSet) UintVar(p *uint, name string, value uint, usage string) {
	f.VarP(newUintValue(value, p), name, "", usage)
}

// Like UintVar, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) UintVarP(p *uint, name, shortcut string, value uint, usage string) {
	f.VarP(newUintValue(value, p), name, shortcut, usage)
}

// UintVar defines a uint flag with specified name, default value, and usage string.
// The argument p points to a uint  variable in which to store the value of the flag.
func UintVar(p *uint, name string, value uint, usage string) {
	commandLine.VarP(newUintValue(value, p), name, "", usage)
}

// Like UintVar, but accepts a shortcut letter that can be used after a single dash.
func UintVarP(p *uint, name, shortcut string, value uint, usage string) {
	commandLine.VarP(newUintValue(value, p), name, shortcut, usage)
}

// Uint defines a uint flag with specified name, default value, and usage string.
// The return value is the address of a uint  variable that stores the value of the flag.
func (f *FlagSet) Uint(name string, value uint, usage string) *uint {
	p := new(uint)
	f.UintVarP(p, name, "", value, usage)
	return p
}

// Like Uint, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) UintP(name, shortcut string, value uint, usage string) *uint {
	p := new(uint)
	f.UintVarP(p, name, shortcut, value, usage)
	return p
}

// Uint defines a uint flag with specified name, default value, and usage string.
// The return value is the address of a uint  variable that stores the value of the flag.
func Uint(name string, value uint, usage string) *uint {
	return commandLine.UintP(name, "", value, usage)
}

// Like Uint, but accepts a shortcut letter that can be used after a single dash.
func UintP(name, shortcut string, value uint, usage string) *uint {
	return commandLine.UintP(name, shortcut, value, usage)
}

// Uint64Var defines a uint64 flag with specified name, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the flag.
func (f *FlagSet) Uint64Var(p *uint64, name string, value uint64, usage string) {
	f.VarP(newUint64Value(value, p), name, "", usage)
}

// Like Uint64Var, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) Uint64VarP(p *uint64, name, shortcut string, value uint64, usage string) {
	f.VarP(newUint64Value(value, p), name, shortcut, usage)
}

// Uint64Var defines a uint64 flag with specified name, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the flag.
func Uint64Var(p *uint64, name string, value uint64, usage string) {
	commandLine.VarP(newUint64Value(value, p), name, "", usage)
}

// Like Uint64Var, but accepts a shortcut letter that can be used after a single dash.
func Uint64VarP(p *uint64, name, shortcut string, value uint64, usage string) {
	commandLine.VarP(newUint64Value(value, p), name, shortcut, usage)
}

// Uint64 defines a uint64 flag with specified name, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the flag.
func (f *FlagSet) Uint64(name string, value uint64, usage string) *uint64 {
	p := new(uint64)
	f.Uint64VarP(p, name, "", value, usage)
	return p
}

// Like Uint64, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) Uint64P(name, shortcut string, value uint64, usage string) *uint64 {
	p := new(uint64)
	f.Uint64VarP(p, name, shortcut, value, usage)
	return p
}

// Uint64 defines a uint64 flag with specified name, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the flag.
func Uint64(name string, value uint64, usage string) *uint64 {
	return commandLine.Uint64P(name, "", value, usage)
}

// Like Uint64, but accepts a shortcut letter that can be used after a single dash.
func Uint64P(name, shortcut string, value uint64, usage string) *uint64 {
	return commandLine.Uint64P(name, shortcut, value, usage)
}

// StringVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a string variable in which to store the value of the flag.
func (f *FlagSet) StringVar(p *string, name string, value string, usage string) {
	f.VarP(newStringValue(value, p), name, "", usage)
}

// Like StringVar, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) StringVarP(p *string, name, shortcut string, value string, usage string) {
	f.VarP(newStringValue(value, p), name, shortcut, usage)
}

// StringVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a string variable in which to store the value of the flag.
func StringVar(p *string, name string, value string, usage string) {
	commandLine.VarP(newStringValue(value, p), name, "", usage)
}

// Like StringVar, but accepts a shortcut letter that can be used after a single dash.
func StringVarP(p *string, name, shortcut string, value string, usage string) {
	commandLine.VarP(newStringValue(value, p), name, shortcut, usage)
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.
func (f *FlagSet) String(name string, value string, usage string) *string {
	p := new(string)
	f.StringVarP(p, name, "", value, usage)
	return p
}

// Like String, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) StringP(name, shortcut string, value string, usage string) *string {
	p := new(string)
	f.StringVarP(p, name, shortcut, value, usage)
	return p
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.
func String(name string, value string, usage string) *string {
	return commandLine.StringP(name, "", value, usage)
}

// Like String, but accepts a shortcut letter that can be used after a single dash.
func StringP(name, shortcut string, value string, usage string) *string {
	return commandLine.StringP(name, shortcut, value, usage)
}

// Float64Var defines a float64 flag with specified name, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the flag.
func (f *FlagSet) Float64Var(p *float64, name string, value float64, usage string) {
	f.VarP(newFloat64Value(value, p), name, "", usage)
}

// Like Float64Var, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) Float64VarP(p *float64, name, shortcut string, value float64, usage string) {
	f.VarP(newFloat64Value(value, p), name, shortcut, usage)
}

// Float64Var defines a float64 flag with specified name, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the flag.
func Float64Var(p *float64, name string, value float64, usage string) {
	commandLine.VarP(newFloat64Value(value, p), name, "", usage)
}

// Like Float64Var, but accepts a shortcut letter that can be used after a single dash.
func Float64VarP(p *float64, name, shortcut string, value float64, usage string) {
	commandLine.VarP(newFloat64Value(value, p), name, shortcut, usage)
}

// Float64 defines a float64 flag with specified name, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the flag.
func (f *FlagSet) Float64(name string, value float64, usage string) *float64 {
	p := new(float64)
	f.Float64VarP(p, name, "", value, usage)
	return p
}

// Like Float64, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) Float64P(name, shortcut string, value float64, usage string) *float64 {
	p := new(float64)
	f.Float64VarP(p, name, shortcut, value, usage)
	return p
}

// Float64 defines a float64 flag with specified name, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the flag.
func Float64(name string, value float64, usage string) *float64 {
	return commandLine.Float64P(name, "", value, usage)
}

// Like Float64, but accepts a shortcut letter that can be used after a single dash.
func Float64P(name, shortcut string, value float64, usage string) *float64 {
	return commandLine.Float64P(name, shortcut, value, usage)
}

// DurationVar defines a time.Duration flag with specified name, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the flag.
func (f *FlagSet) DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	f.VarP(newDurationValue(value, p), name, "", usage)
}

// Like DurationVar, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) DurationVarP(p *time.Duration, name, shortcut string, value time.Duration, usage string) {
	f.VarP(newDurationValue(value, p), name, shortcut, usage)
}

// DurationVar defines a time.Duration flag with specified name, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the flag.
func DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	commandLine.VarP(newDurationValue(value, p), name, "", usage)
}

// Like DurationVar, but accepts a shortcut letter that can be used after a single dash.
func DurationVarP(p *time.Duration, name, shortcut string, value time.Duration, usage string) {
	commandLine.VarP(newDurationValue(value, p), name, shortcut, usage)
}

// Duration defines a time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the flag.
func (f *FlagSet) Duration(name string, value time.Duration, usage string) *time.Duration {
	p := new(time.Duration)
	f.DurationVarP(p, name, "", value, usage)
	return p
}

// Like Duration, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) DurationP(name, shortcut string, value time.Duration, usage string) *time.Duration {
	p := new(time.Duration)
	f.DurationVarP(p, name, shortcut, value, usage)
	return p
}

// Duration defines a time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the flag.
func Duration(name string, value time.Duration, usage string) *time.Duration {
	return commandLine.DurationP(name, "", value, usage)
}

// Like Duration, but accepts a shortcut letter that can be used after a single dash.
func DurationP(name, shortcut string, value time.Duration, usage string) *time.Duration {
	return commandLine.DurationP(name, shortcut, value, usage)
}

// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.
func (f *FlagSet) Var(value Value, name string, usage string) {
	f.VarP(value, name, "", usage)
}

// Like Var, but accepts a shortcut letter that can be used after a single dash.
func (f *FlagSet) VarP(value Value, name, shortcut, usage string) {
	// Remember the default value as a string; it won't change.
	flag := &Flag{name, shortcut, usage, value, value.String()}
	_, alreadythere := f.formal[name]
	if alreadythere {
		fmt.Fprintf(f.out(), "%s flag redefined: %s\n", f.name, name)
		panic("flag redefinition") // Happens only if flags are declared with identical names
	}
	if f.formal == nil {
		f.formal = make(map[string]*Flag)
	}
	f.formal[name] = flag

	if len(shortcut) == 0 {
		return
	}
	if len(shortcut) > 1 {
		fmt.Fprintf(f.out(), "%s shortcut more than ASCII character: %s\n", f.name, shortcut)
		panic("shortcut is more than one character")
	}
	if f.shortcuts == nil {
		f.shortcuts = make(map[byte]*Flag)
	}
	c := shortcut[0]
	old, alreadythere := f.shortcuts[c]
	if alreadythere {
		fmt.Fprintf(f.out(), "%s shortcut reused: %q for %s and %s\n", f.name, c, name, old.Name)
		panic("shortcut redefinition")
	}
	f.shortcuts[c] = flag
}


// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.
func Var(value Value, name string, usage string) {
	commandLine.VarP(value, name, "", usage)
}

// failf prints to standard error a formatted error and usage message and
// returns the error.
func (f *FlagSet) failf(format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintln(f.out(), err)
	f.usage()
	return err
}

// usage calls the Usage method for the flag set, or the usage function if
// the flag set is commandLine.
func (f *FlagSet) usage() {
	if f == commandLine {
		Usage()
	} else if f.Usage == nil {
		defaultUsage(f)
	} else {
		f.Usage()
	}
}

func (f *FlagSet) parseArgs(args []string) error {
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		if len(s) == 0 || s[0] != '-' || len(s) == 1 {
			f.args = append(f.args, s)
			continue
		}

		var flag *Flag = nil
		has_value := false
		value := ""
		if s[1] == '-' {
			if len(s) == 2 { // "--" terminates the flags
				f.args = append(f.args, args...)
				return nil
			}
			name := s[2:]
			if len(name) == 0 || name[0] == '-' || name[0] == '=' {
				return f.failf("bad flag syntax: %s", s)
			}
			// check for = argument to flag
			for i := 1; i < len(name); i++ { // equals cannot be first
				if name[i] == '=' {
					value = name[i+1:]
					has_value = true
					name = name[0:i]
					break
				}
			}
			m := f.formal
			_, alreadythere := m[name] // BUG
			if !alreadythere {
				if name == "help" { // special case for nice help message.
					f.usage()
					return ErrHelp
				}
				return f.failf("flag provided but not defined: --%s", name)
			}
			flag = m[name]
		} else {
			shortcuts := s[1:]
			for i := 0; i < len(shortcuts); i++ {
				c := shortcuts[i]
				_, alreadythere := f.shortcuts[c]
				if !alreadythere {
					if c == 'h' { // special case for nice help message.
						f.usage()
						return ErrHelp
					}
					return f.failf("flag provided but not defined: %q in -%s", c, shortcuts)
				}
				flag = f.shortcuts[c]
				if i == len(shortcuts) - 1 {
					break
				}
				if shortcuts[i+1] == '=' {
					value = shortcuts[i+2:]
					has_value = true
					break
				}
				if fv, ok := flag.Value.(*boolValue); ok {
					fv.Set("true")
				} else {
					value = shortcuts[i+1:]
					has_value = true
					break
				}
			}
		}

		// we have a flag, possibly with included =value argument
		if fv, ok := flag.Value.(*boolValue); ok { // special case: doesn't need an arg
			if has_value {
				if err := fv.Set(value); err != nil {
					f.failf("invalid boolean value %q for %s: %v", value, s, err)
				}
			} else {
				fv.Set("true")
			}
		} else {
			// It must have a value, which might be the next argument.
			if !has_value && len(args) > 0 {
				// value is the next arg
				has_value = true
				value = args[0]
				args = args[1:]
			}
			if !has_value {
				return f.failf("flag needs an argument: %s", s)
			}
			if err := flag.Value.Set(value); err != nil {
				return f.failf("invalid value %q for %s: %v", value, s, err)
			}
		}
		/*if f.actual == nil {
			f.actual = make(map[string]*Flag)
		}
		f.actual[name] = flag*/ // TODO: mark flags as set in robust way
	}
	return nil
}

// Parse parses flag definitions from the argument list, which should not
// include the command name.  Must be called after all flags in the FlagSet
// are defined and before flags are accessed by the program.
// The return value will be ErrHelp if -help was set but not defined.
func (f *FlagSet) Parse(arguments []string) error {
	f.parsed = true
	f.args = make([]string, 0, len(arguments))
	err := f.parseArgs(arguments)
	if err != nil {
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
	}
	return nil
}

// Parsed reports whether f.Parse has been called.
func (f *FlagSet) Parsed() bool {
	return f.parsed
}

// Parse parses the command-line flags from os.Args[1:].  Must be called
// after all flags are defined and before flags are accessed by the program.
func Parse() {
	// Ignore errors; commandLine is set for ExitOnError.
	commandLine.Parse(os.Args[1:])
}

// Parsed returns true if the command-line flags have been parsed.
func Parsed() bool {
	return commandLine.Parsed()
}

// The default set of command-line flags, parsed from os.Args.
var commandLine = NewFlagSet(os.Args[0], ExitOnError)

// NewFlagSet returns a new, empty flag set with the specified name and
// error handling property.
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := &FlagSet{
		name:          name,
		errorHandling: errorHandling,
	}
	return f
}

// Init sets the name and error handling property for a flag set.
// By default, the zero FlagSet uses an empty name and the
// ContinueOnError error handling policy.
func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.name = name
	f.errorHandling = errorHandling
}
