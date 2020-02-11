package flagx

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/henrylee2cn/ameda"
	"github.com/henrylee2cn/goutil"
)

type (
	// ErrorHandling defines how FlagSet.Parse behaves if the parse fails.
	ErrorHandling = flag.ErrorHandling

	// A FlagSet represents a set of defined flags. The zero value of a FlagSet
	// has no name and has ContinueOnError error handling.
	FlagSet struct {
		*flag.FlagSet
		errorHandling         ErrorHandling
		isContinueOnUndefined bool
		terminated            bool
		nonActual             map[int]*Flag
		nonFormal             map[int]*Flag
	}

	// A Flag represents the state of a flag.
	Flag = flag.Flag

	// Getter is an interface that allows the contents of a Value to be retrieved.
	// It wraps the Value interface, rather than being part of it, because it
	// appeared after Go 1 and its compatibility rules. All Value types provided
	// by this package satisfy the Getter interface.
	Getter = flag.Getter

	// Value is the interface to the dynamic value stored in a flag.
	// (The default value is represented as a string.)
	//
	// If a Value has an IsBoolFlag() bool method returning true,
	// the command-line parser makes -name equivalent to -name=true
	// rather than using the next command-line argument.
	//
	// Set is called once, in command line order, for each flag present.
	// The flag package may call the String method with a zero-valued receiver,
	// such as a nil pointer.
	Value = flag.Value
)

// These constants cause FlagSet.Parse to behave as described if the parse fails.
const (
	ContinueOnError     ErrorHandling = flag.ContinueOnError // Return a descriptive error.
	ExitOnError         ErrorHandling = flag.ExitOnError     // Call os.Exit(2).
	PanicOnError        ErrorHandling = flag.PanicOnError    // Call panic with a descriptive error.
	ContinueOnUndefined ErrorHandling = 1 << 30              // Ignore provided but undefined flags
)

// NewFlagSet returns a new, empty flag set with the specified name and
// error handling property. If the name is not empty, it will be printed
// in the default usage message and in error messages.
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := new(FlagSet)
	f.Init(name, errorHandling)
	return f
}

// Init sets the name and error handling property for a flag set.
// By default, the zero FlagSet uses an empty name and the
// ContinueOnError error handling policy.
func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.errorHandling = errorHandling
	errorHandling, f.isContinueOnUndefined = cleanBit(errorHandling, ContinueOnUndefined)
	if f.FlagSet == nil {
		f.FlagSet = flag.NewFlagSet(name, errorHandling)
	} else {
		f.FlagSet.Init(name, errorHandling)
	}
}

// ErrorHandling returns the error handling behavior of the flag set.
func (f *FlagSet) ErrorHandling() ErrorHandling {
	return f.errorHandling
}

// SubArgs returns arguments of the next subcommand.
func (f *FlagSet) SubArgs() []string {
	if f.terminated {
		return f.Args()
	}
	args := f.Args()
	idx := ameda.NewStringSlice(args).IndexOf("--")
	if idx >= 0 {
		return args[idx+1:]
	}
	return nil
}

// StructVars defines flags based on struct tags and binds to fields.
// NOTE:
//  Not support nested fields
func (f *FlagSet) StructVars(p interface{}) error {
	v := reflect.ValueOf(p)
	if v.Kind() == reflect.Ptr {
		v = goutil.DereferenceValue(v)
		if v.Kind() == reflect.Struct {
			structTypeIDs := make(map[int32]struct{}, 4)
			return f.varFromStruct(v, structTypeIDs)
		}
	}
	return fmt.Errorf("flagx: want struct pointer parameter, but got %T", p)
}

// BoolNonVar defines a bool non-flag with specified index, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the non-flag.
func (f *FlagSet) BoolNonVar(p *bool, index int, value bool, usage string) {
	f.NonVar(newBoolValue(value, p), index, usage)
}

// BoolNon defines a bool non-flag with specified index, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the non-flag.
func (f *FlagSet) BoolNon(index int, value bool, usage string) *bool {
	p := new(bool)
	f.BoolNonVar(p, index, value, usage)
	return p
}

// IntNonVar defines an int non-flag with specified index, default value, and usage string.
// The argument p points to an int variable in which to store the value of the non-flag.
func (f *FlagSet) IntNonVar(p *int, index int, value int, usage string) {
	f.NonVar(newIntValue(value, p), index, usage)
}

// IntNon defines an int non-flag with specified index, default value, and usage string.
// The return value is the address of an int variable that stores the value of the non-flag.
func (f *FlagSet) IntNon(index int, value int, usage string) *int {
	p := new(int)
	f.IntNonVar(p, index, value, usage)
	return p
}

// Int64NonVar defines an int64 non-flag with specified index, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the non-flag.
func (f *FlagSet) Int64NonVar(p *int64, index int, value int64, usage string) {
	f.NonVar(newInt64Value(value, p), index, usage)
}

// Int64Non defines an int64 non-flag with specified index, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the non-flag.
func (f *FlagSet) Int64Non(index int, value int64, usage string) *int64 {
	p := new(int64)
	f.Int64NonVar(p, index, value, usage)
	return p
}

// UintNonVar defines a uint non-flag with specified index, default value, and usage string.
// The argument p points to a uint variable in which to store the value of the non-flag.
func (f *FlagSet) UintNonVar(p *uint, index int, value uint, usage string) {
	f.NonVar(newUintValue(value, p), index, usage)
}

// UintNon defines a uint non-flag with specified index, default value, and usage string.
// The return value is the address of a uint variable that stores the value of the non-flag.
func (f *FlagSet) UintNon(index int, value uint, usage string) *uint {
	p := new(uint)
	f.UintNonVar(p, index, value, usage)
	return p
}

// Uint64NonVar defines a uint64 non-flag with specified index, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the non-flag.
func (f *FlagSet) Uint64NonVar(p *uint64, index int, value uint64, usage string) {
	f.NonVar(newUint64Value(value, p), index, usage)
}

// Uint64Non defines a uint64 non-flag with specified index, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the non-flag.
func (f *FlagSet) Uint64Non(index int, value uint64, usage string) *uint64 {
	p := new(uint64)
	f.Uint64NonVar(p, index, value, usage)
	return p
}

// StringNonVar defines a string non-flag with specified index, default value, and usage string.
// The argument p points to a string variable in which to store the value of the non-flag.
func (f *FlagSet) StringNonVar(p *string, index int, value string, usage string) {
	f.NonVar(newStringValue(value, p), index, usage)
}

// StringNon defines a string non-flag with specified index, default value, and usage string.
// The return value is the address of a string variable that stores the value of the non-flag.
func (f *FlagSet) StringNon(index int, value string, usage string) *string {
	p := new(string)
	f.StringNonVar(p, index, value, usage)
	return p
}

// Float64NonVar defines a float64 non-flag with specified index, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the non-flag.
func (f *FlagSet) Float64NonVar(p *float64, index int, value float64, usage string) {
	f.NonVar(newFloat64Value(value, p), index, usage)
}

// Float64Non defines a float64 non-flag with specified index, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the non-flag.
func (f *FlagSet) Float64Non(index int, value float64, usage string) *float64 {
	p := new(float64)
	f.Float64NonVar(p, index, value, usage)
	return p
}

// DurationNonVar defines a time.Duration non-flag with specified index, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the non-flag.
// The non-flag accepts a value acceptable to time.ParseDuration.
func (f *FlagSet) DurationNonVar(p *time.Duration, index int, value time.Duration, usage string) {
	f.NonVar(newDurationValue(value, p), index, usage)
}

// DurationNon defines a time.Duration non-flag with specified index, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the non-flag.
// The non-flag accepts a value acceptable to time.ParseDuration.
func (f *FlagSet) DurationNon(index int, value time.Duration, usage string) *time.Duration {
	p := new(time.Duration)
	f.DurationNonVar(p, index, value, usage)
	return p
}

// NonVar defines a non-flag with the specified index and usage string.
func (f *FlagSet) NonVar(value Value, index int, usage string) {
	if index < 0 {
		panic("@index is not a valid slice index")
	}
	name := getNonFlagName(index)
	// Remember the default value as a string; it won't change.
	flag := &Flag{name, usage, value, value.String()}
	_, alreadythere := f.nonFormal[index]
	if alreadythere {
		var msg string
		if f.Name() == "" {
			msg = fmt.Sprintf("flag redefined: %s", name)
		} else {
			msg = fmt.Sprintf("%s flag redefined: %s", f.Name(), name)
		}
		fmt.Fprintln(f.Output(), msg)
		panic(msg) // Happens only if flags are declared with identical names
	}
	if f.nonFormal == nil {
		f.nonFormal = make(map[int]*Flag)
	}
	f.nonFormal[index] = flag
}

// Parse parses flag definitions from the argument list, which should not
// include the command name. Must be called after all flags in the FlagSet
// are defined and before flags are accessed by the program.
// The return value will be ErrHelp if -help or -h were set but not defined.
func (f *FlagSet) Parse(arguments []string) error {
	_, arguments = SplitArgs(arguments)
	if f.isContinueOnUndefined {
		flagArgs, nonFlagArgs, terminated, err := tidyArgs(arguments, func(name string) (want, next bool) {
			return f.FlagSet.Lookup(name) != nil, true
		})
		if err != nil {
			return err
		}
		arguments = make([]string, 0, len(flagArgs)+len(nonFlagArgs)+1)
		arguments = append(arguments, flagArgs...)
		if terminated {
			arguments = append(arguments, "--")
		}
		arguments = append(arguments, nonFlagArgs...)
		f.terminated = terminated
	}
	err := f.FlagSet.Parse(arguments)
	if err != nil {
		return err
	}
	if f.terminated {
		return nil
	}

	args := f.Args()
	if !f.isContinueOnUndefined {
		if len(args) == 0 {
			return nil
		}
		i := len(arguments) - len(args)
		if i > 0 {
			i -= 1
		}
		if arguments[i] == "--" {
			f.terminated = true
			return nil
		}
	}

	for k, v := range args {
		seen, err := f.parseOneNonFlag(k, v)
		if seen {
			continue
		}
		if err == nil {
			break
		}
		switch f.FlagSet.ErrorHandling() {
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

// parseOneNonFlag parses one non-flag. It reports whether a non-flag was seen.
func (f *FlagSet) parseOneNonFlag(index int, value string) (bool, error) {
	if value == "--" {
		return false, f.failf("non-flag defined but not provided: %d", index)
	}
	m := f.nonFormal
	flag, alreadythere := m[index]
	if !alreadythere {
		return false, nil
		// return false, f.failf("non-flag provided but not defined: %d", index)
	}
	if err := flag.Value.Set(value); err != nil {
		return false, f.failf("invalid value %q for non-flag %d: %v", value, index, err)
	}
	if f.nonActual == nil {
		f.nonActual = make(map[int]*Flag)
	}
	f.nonActual[index] = flag
	return true, nil
}

// failf prints to standard error a formatted error and usage message and
// returns the error.
func (f *FlagSet) failf(format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintln(f.Output(), err)
	f.usage()
	return err
}

// usage calls the Usage method for the flag set if one is specified,
// or the appropriate default usage function otherwise.
func (f *FlagSet) usage() {
	if f.Usage == nil {
		f.defaultUsage()
	} else {
		f.Usage()
	}
}

// defaultUsage is the default function to print a usage message.
func (f *FlagSet) defaultUsage() {
	if f.Name() == "" {
		fmt.Fprintf(f.Output(), "Usage:\n")
	} else {
		fmt.Fprintf(f.Output(), "Usage of %s:\n", f.Name())
	}
	f.PrintDefaults()
}

func tidyArgs(args []string, filter func(name string) (want, next bool)) (tidiedArgs, lastArgs []string, terminated bool, err error) {
	tidiedArgs = make([]string, 0, len(args)*2)
	lastArgs, terminated, err = filterArgs(args, func(name string, valuePtr *string) bool {
		want, next := filter(name)
		if want {
			var kv []string
			if valuePtr == nil {
				kv = []string{"-" + name}
			} else {
				kv = []string{"-" + name, *valuePtr}
			}
			tidiedArgs = append(tidiedArgs, kv...)
		}
		return next
	})
	return tidiedArgs, lastArgs, terminated, err
}

func filterArgs(args []string, filter func(name string, valuePtr *string) (next bool)) (lastArgs []string, terminated bool, err error) {
	lastArgs = args
	var name string
	var valuePtr *string
	var seen bool
	for {
		lastArgs, terminated, name, valuePtr, seen, err = tidyOneArg(lastArgs)
		if !seen {
			return
		}
		next := filter(name, valuePtr)
		if !next {
			return
		}
	}
}

// tidyOneArg tidies one flag. It reports whether a flag was seen.
func tidyOneArg(args []string) (lastArgs []string, terminated bool, name string, valuePtr *string, seen bool, err error) {
	if len(args) == 0 {
		lastArgs = args
		return
	}
	s := args[0]
	if len(s) < 2 || s[0] != '-' {
		lastArgs = args
		return
	}
	numMinuses := 1
	if s[1] == '-' {
		numMinuses++
		if len(s) == 2 { // "--" terminates the flags
			lastArgs = args[1:]
			terminated = true
			return
		}
	}
	name = s[numMinuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		err = fmt.Errorf("bad flag syntax: %s", s)
		lastArgs = args
		return
	}

	// it's a flag.
	seen = true
	args = args[1:]

	// does it have an argument?
	for i := 1; i < len(name); i++ { // equals cannot be first
		if name[i] == '=' {
			value := name[i+1:]
			valuePtr = &value
			name = name[0:i]
			lastArgs = args
			return
		}
	}

	// doesn't have an arg
	if len(args) == 0 {
		lastArgs = args
		return
	}

	// value is the next arg
	if maybeValue := args[0]; len(maybeValue) == 0 || maybeValue[0] != '-' {
		valuePtr = &maybeValue
		lastArgs = args[1:]
		return
	}

	// doesn't have an arg
	lastArgs = args
	return
}

func cleanBit(eh, bit ErrorHandling) (ErrorHandling, bool) {
	eh2 := eh &^ bit
	return eh2, eh2 != eh
}

func getNonFlagName(index int) string {
	return tagKeyNonFlag + strconv.Itoa(index)
}

func getNonFlagIndex(name string) (int, bool, error) {
	s := strings.TrimPrefix(name, tagKeyNonFlag)
	if s == name {
		return 0, false, nil
	}
	i, err := ameda.StringToInt(s, true)
	return i, true, err
}
