package pflag

import "fmt"

// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.
func (f *FlagSet) Var(value Value, name string, usage string) {
	f.VarP(value, name, "", usage)
}

// Like Var, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) VarP(value Value, name, shorthand, usage string) {
	// Remember the default value as a string; it won't change.
	flag := &Flag{name, shorthand, usage, value, value.String()}
	_, alreadythere := f.formal[name]
	if alreadythere {
		msg := fmt.Sprintf("%s flag redefined: %s", f.name, name)
		fmt.Fprintln(f.out(), msg)
		panic(msg) // Happens only if flags are declared with identical names
	}
	if f.formal == nil {
		f.formal = make(map[string]*Flag)
	}
	f.formal[name] = flag

	if len(shorthand) == 0 {
		return
	}
	if len(shorthand) > 1 {
		fmt.Fprintf(f.out(), "%s shorthand more than ASCII character: %s\n", f.name, shorthand)
		panic("shorthand is more than one character")
	}
	if f.shorthands == nil {
		f.shorthands = make(map[byte]*Flag)
	}
	c := shorthand[0]
	old, alreadythere := f.shorthands[c]
	if alreadythere {
		fmt.Fprintf(f.out(), "%s shorthand reused: %q for %s and %s\n", f.name, c, name, old.Name)
		panic("shorthand redefinition")
	}
	f.shorthands[c] = flag
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

// Like Var, but accepts a shorthand letter that can be used after a single dash.
func VarP(value Value, name, shorthand, usage string) {
	commandLine.VarP(value, name, shorthand, usage)
}
