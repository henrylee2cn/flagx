package flagx

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/henrylee2cn/ameda"
	"github.com/henrylee2cn/goutil/status"
)

type (
	// App is a application structure. It is recommended that
	// an app be created with the flagx.NewApp() function
	App struct {
		*Command
		appName       string
		version       string
		compiled      time.Time
		authors       []Author
		copyright     string
		notFound      ActionFunc
		usageText     string
		usageTemplate *template.Template
		validator     ValidateFunc
		lock          sync.RWMutex
	}
	// Command a command object
	Command struct {
		app               *App
		parent            *Command
		cmdName           string
		description       string
		filters           []*filterObject
		action            *actionObject
		subcommands       map[string]*Command
		sortedSubCommands []*Command
		usageBody         string
		usageText         string
		lock              sync.RWMutex
	}
	// ValidateFunc validator for struct flag
	ValidateFunc func(interface{}) error
	// Action action of action
	Action interface {
		// Handle handles arguments.
		// NOTE:
		//  If need to return an error, use *Context.ThrowStatus or *Context.CheckStatus
		Handle(*Context)
	}
	// ActionCopier an interface that can create its own copy
	ActionCopier interface {
		DeepCopy() Action
	}
	// FilterCopier an interface that can create its own copy
	FilterCopier interface {
		DeepCopy() Filter
	}
	// ActionFunc action function
	// NOTE:
	//  If need to return an error, use *Context.ThrowStatus or *Context.CheckStatus
	ActionFunc func(*Context)
	// Filter global options of app
	// NOTE:
	//  If need to return an error, use *Context.ThrowStatus or *Context.CheckStatus
	Filter interface {
		Filter(c *Context, next ActionFunc)
	}
	// FilterFunc filter function
	// NOTE:
	//  If need to return an error, use *Context.ThrowStatus or *Context.CheckStatus
	FilterFunc func(c *Context, next ActionFunc)
	// Context context of an action execution
	Context struct {
		context.Context
		args    []string
		cmdPath []string
	}
	// Author represents someone who has contributed to a cli project.
	Author struct {
		Name  string // The Authors name
		Email string // The Authors email
	}
	// Status a handling status with code, msg, cause and stack.
	Status = status.Status
)

type (
	contextKey    int8
	actionFactory struct {
		elemType reflect.Type
	}
	factory      actionFactory
	actionObject struct {
		cmd           *Command
		flagSet       *FlagSet
		options       map[string]*Flag
		actionFactory ActionCopier
		actionFunc    ActionFunc
	}
	filterObject struct {
		flagSet    *FlagSet
		options    map[string]*Flag
		factory    FilterCopier
		filterFunc FilterFunc
	}
)

// Status code
const (
	StatusBadArgs        int32 = 1
	StatusNotFound       int32 = 2
	StatusParseFailed    int32 = 3
	StatusValidateFailed int32 = 4
)

const (
	currCmdName contextKey = iota
)

var (
	// NewStatus creates a message status with code, msg and cause.
	// NOTE:
	//  code=0 means no error
	// TYPE:
	//  func NewStatus(code int32, msg string, cause interface{}) *Status
	NewStatus = status.New

	// NewStatusWithStack creates a message status with code, msg and cause and stack.
	// NOTE:
	//  code=0 means no error
	// TYPE:
	//  func NewStatusWithStack(code int32, msg string, cause interface{}) *Status
	NewStatusWithStack = status.NewWithStack

	// NewStatusFromQuery parses the query bytes to a status object.
	// TYPE:
	//  func NewStatusFromQuery(b []byte, tagStack bool) *Status
	NewStatusFromQuery = status.FromQuery
	// CheckStatus if err!=nil, create a status with stack, and panic.
	// NOTE:
	//  If err!=nil and msg=="", error text is set to msg
	// TYPE:
	//  func Check(err error, code int32, msg string, whenError ...func())
	CheckStatus = status.Check
	// ThrowStatus creates a status with stack, and panic.
	// TYPE:
	//  func Throw(code int32, msg string, cause interface{})
	ThrowStatus = status.Throw
	// PanicStatus panic with stack trace.
	// TYPE:
	//  func Panic(stat *Status)
	PanicStatus = status.Panic
	// CatchStatus recovers the panic and returns status.
	// NOTE:
	//  Set `realStat` to true if a `Status` type is recovered
	// Example:
	//  var stat *Status
	//  defer Catch(&stat)
	// TYPE:
	//  func Catch(statPtr **Status, realStat ...*bool)
	CatchStatus = status.Catch
)

// NewApp creates a new application.
func NewApp() *App {
	a := new(App)
	return a.init()
}

func (a *App) init() *App {
	a.Command = newCommand(a, "", "")
	a.SetCmdName("")
	a.SetName("")
	a.SetVersion("")
	a.SetCompiled(time.Time{})
	a.SetUsageTemplate(defaultAppUsageTemplate)
	return a
}

func newCommand(app *App, cmdName, description string) *Command {
	return &Command{
		app:         app,
		cmdName:     cmdName,
		description: description,
		subcommands: make(map[string]*Command, 16),
	}
}

// CmdName returns the command name of the application.
// Defaults to filepath.Base(os.Args[0])
func (a *App) CmdName() string {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.cmdName
}

// SetCmdName sets the command name of the application.
// NOTE:
//  remove '-' prefix automatically
func (a *App) SetCmdName(cmdName string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	if cmdName == "" {
		cmdName = filepath.Base(os.Args[0])
	}
	a.cmdName = strings.TrimLeft(cmdName, "-")
	a.updateUsageLocked()
}

// Name returns the name(title) of the application.
// Defaults to *App.CmdName()
func (a *App) Name() string {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.appName == "" {
		return a.cmdName
	}
	return a.appName
}

// SetName sets the name(title) of the application.
func (a *App) SetName(appName string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.appName = appName
	a.updateUsageLocked()
}

// Description returns description the of the application.
func (a *App) Description() string {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.description
}

// SetDescription sets description the of the application.
func (a *App) SetDescription(description string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.description = description
	a.updateUsageLocked()
}

// Version returns the version of the application.
func (a *App) Version() string {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.version
}

// SetVersion sets the version of the application.
func (a *App) SetVersion(version string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	if version == "" {
		version = "0.0.1"
	}
	a.version = version
	a.updateUsageLocked()
}

// Compiled returns the compilation date.
func (a *App) Compiled() time.Time {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.compiled
}

// SetCompiled sets the compilation date.
func (a *App) SetCompiled(date time.Time) {
	a.lock.Lock()
	defer a.lock.Unlock()
	if date.IsZero() {
		info, err := os.Stat(os.Args[0])
		if err != nil {
			date = time.Now()
		} else {
			date = info.ModTime()
		}
	}
	a.compiled = date
	a.updateUsageLocked()
}

// Authors returns the list of all authors who contributed.
func (a *App) Authors() []Author {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.authors
}

// SetAuthors sets the list of all authors who contributed.
func (a *App) SetAuthors(authors []Author) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.authors = authors
	a.updateUsageLocked()
}

// Copyright returns the copyright of the binary if any.
func (a *App) Copyright() string {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.copyright
}

// SetCopyright sets copyright of the binary if any.
func (a *App) SetCopyright(copyright string) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.copyright = copyright
	a.updateUsageLocked()
}

// Handle implements Action interface.
func (fn ActionFunc) Handle(c *Context) {
	fn(c)
}

// Filter implements Filter interface.
func (fn FilterFunc) Filter(c *Context, next ActionFunc) {
	fn(c, next)
}

// SetValidator sets the validation function.
func (a *App) SetValidator(validator ValidateFunc) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.validator = validator
}

// AddSubaction adds a subcommand and its action.
// NOTE:
//  panic when something goes wrong
func (c *Command) AddSubaction(cmdName, description string, action Action, filters ...Filter) {
	c.AddSubcommand(cmdName, description, filters...).SetAction(action)
}

// AddSubcommand adds a subcommand.
// NOTE:
//  panic when something goes wrong
func (c *Command) AddSubcommand(cmdName, description string, filters ...Filter) *Command {
	if c.action != nil {
		panic(fmt.Errorf("action has been set, no subcommand can be set: %q", c.pathString()))
	}
	if cmdName == "" {
		panic("command name is empty")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.subcommands[cmdName] != nil {
		panic(fmt.Errorf("action named %s already exists", cmdName))
	}
	subCmd := newCommand(c.app, cmdName, description)
	subCmd.parent = c
	for _, filter := range filters {
		subCmd.AddFilter(filter)
	}
	c.subcommands[cmdName] = subCmd
	return subCmd
}

// AddFilter adds the filter action.
// NOTE:
//  if filter is a struct, it can implement the copier interface;
//  panic when something goes wrong
func (c *Command) AddFilter(filter Filter) {
	c.lock.Lock()
	defer c.lock.Unlock()
	var obj filterObject
	obj.flagSet = NewFlagSet(c.cmdName, ContinueOnError|ContinueOnUndefined)

	elemType := ameda.DereferenceType(reflect.TypeOf(filter))
	switch elemType.Kind() {
	case reflect.Struct:
		var ok bool
		obj.factory, ok = filter.(FilterCopier)
		if !ok {
			obj.factory = &factory{elemType: elemType}
		}
		err := obj.flagSet.StructVars(obj.factory.DeepCopy())
		if err != nil {
			panic(err)
		}
		obj.flagSet.VisitAll(func(f *Flag) {
			if obj.options == nil {
				obj.options = make(map[string]*Flag)
			}
			obj.options[f.Name] = f
		})
	case reflect.Func:
		obj.filterFunc = filter.Filter
	}
	c.filters = append(c.filters, &obj)
	c.updateUsageLocked()
}

// SetAction sets the action of the command.
// NOTE:
//  if action is a struct, it can implement the copier interface;
//  panic when something goes wrong.
func (c *Command) SetAction(action Action) {
	if len(c.subcommands) > 0 {
		panic(fmt.Errorf("some subcommands have been set, no action can be set: %q", c.pathString()))
	}
	var obj actionObject
	obj.cmd = c
	obj.flagSet = NewFlagSet(c.cmdName, ContinueOnError|ContinueOnUndefined)

	elemType := ameda.DereferenceType(reflect.TypeOf(action))
	switch elemType.Kind() {
	case reflect.Struct:
		var ok bool
		obj.actionFactory, ok = action.(ActionCopier)
		if !ok {
			obj.actionFactory = &actionFactory{elemType: elemType}
		}
		err := obj.flagSet.StructVars(obj.actionFactory.DeepCopy())
		if err != nil {
			panic(err)
		}
		obj.flagSet.VisitAll(func(f *Flag) {
			if obj.options == nil {
				obj.options = make(map[string]*Flag)
			}
			obj.options[f.Name] = f
		})
	case reflect.Func:
		obj.actionFunc = action.Handle
	}
	c.action = &obj
	c.updateUsageLocked()
}

func (c *Command) path() (p []string) {
	for {
		if c.parent == nil {
			p = append(p, c.cmdName)
			ameda.NewStringSlice(p).Reverse()
			return
		}
		p = append(p, c.cmdName, c.parent.cmdName)
	}
}

func (c *Command) pathString() string {
	return strings.Join(c.path(), " ")
}

// SetNotFound sets the action when the correct command cannot be found.
func (a *App) SetNotFound(fn ActionFunc) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.notFound = fn
}

// SetDefaultValidator sets the default validator of struct flag.
func (a *App) SetDefaultValidator(fn ValidateFunc) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.validator = fn
}

// SetUsageTemplate sets usage template.
func (a *App) SetUsageTemplate(tmpl *template.Template) {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.usageTemplate = tmpl
}

// Exec executes application based on the arguments.
func (a *App) Exec(ctx context.Context, arguments []string) (stat *Status) {
	defer status.Catch(&stat)
	handle, ctxObj := a.route(ctx, arguments)
	handle(ctxObj)
	return
}

func (a *App) route(ctx context.Context, arguments []string) (ActionFunc, *Context) {
	a.lock.RLock()
	defer a.lock.RUnlock()
	filters, action, cmdPath, found := a.Command.findFiltersAndAction([]string{a.cmdName}, arguments)
	actionFunc := action.Handle
	if found {

		for i := len(filters) - 1; i >= 0; i-- {
			filter := filters[i]
			nextHandle := actionFunc
			actionFunc = func(c *Context) {
				filter.Filter(c, nextHandle)
			}
		}
	}
	return actionFunc, &Context{args: arguments, cmdPath: cmdPath, Context: ctx}
}

func (c *Command) findFiltersAndAction(cmdPath, arguments []string) ([]Filter, Action, []string, bool) {
	filters, arguments := c.newFilters(arguments)
	action, arguments, found := c.newAction(arguments)
	if found {
		return filters, action, cmdPath, true
	}
	subCmdName, arguments := SplitArgs(arguments)
	subCmd := c.subcommands[subCmdName]
	if subCmdName != "" {
		cmdPath = append(cmdPath, subCmdName)
	}
	if subCmd == nil {
		if c.app.notFound != nil {
			return nil, c.app.notFound, cmdPath, false
		}
		ThrowStatus(
			StatusNotFound,
			"",
			fmt.Sprintf("not found command action: %q", strings.Join(cmdPath, " ")),
		)
		return nil, nil, cmdPath, false
	}
	subFilters, action, cmdPath, found := subCmd.findFiltersAndAction(cmdPath, arguments)
	if found {
		filters = append(filters, subFilters...)
		return filters, action, cmdPath, true
	}
	return nil, action, cmdPath, false
}

func (c *Command) newFilters(arguments []string) (r []Filter, args []string) {
	r = make([]Filter, len(c.filters))
	args = arguments
	for i, filter := range c.filters {
		if filter.filterFunc != nil {
			r[i] = filter.filterFunc
		} else {
			flagSet := NewFlagSet(c.cmdName, filter.flagSet.ErrorHandling())
			newObj := filter.factory.DeepCopy()
			flagSet.StructVars(newObj)
			err := flagSet.Parse(arguments)
			CheckStatus(err, StatusParseFailed, "")
			if c.app.validator != nil {
				err = c.app.validator(newObj)
			}
			CheckStatus(err, StatusValidateFailed, "")
			r[i] = newObj
			nargs := flagSet.NextArgs()
			if len(args) > len(nargs) {
				args = nargs
			}
		}
	}
	return r, args
}

func (c *Command) newAction(cmdline []string) (Action, []string, bool) {
	a := c.action
	if a == nil {
		return nil, cmdline, false
	}
	cmdName := a.flagSet.Name()
	if a.actionFunc != nil {
		_, cmdline = SplitArgs(cmdline)
		return a.actionFunc, cmdline, true
	}
	flagSet := NewFlagSet(cmdName, a.flagSet.ErrorHandling())
	newObj := a.actionFactory.DeepCopy()
	flagSet.StructVars(newObj)
	err := flagSet.Parse(cmdline)
	CheckStatus(err, StatusParseFailed, "")
	if a.cmd.app.validator != nil {
		err = a.cmd.app.validator(newObj)
	}
	CheckStatus(err, StatusValidateFailed, "")
	return newObj.(Action), flagSet.NextArgs(), true
}

// UsageText returns the usage text.
func (a *App) UsageText() string {
	if a.CmdName() == "" { // not initialized with flagx.NewApp()
		a.init()
	}
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.usageText
}

func (h *actionFactory) DeepCopy() Action {
	return reflect.New(h.elemType).Interface().(Action)
}

func (f *factory) DeepCopy() Filter {
	return reflect.New(f.elemType).Interface().(Filter)
}

// UsageText returns the usage text.
func (c *Command) UsageText(prefix ...string) string {
	if len(prefix) > 0 {
		return strings.Replace(c.usageText, "\n", "\n"+prefix[0], -1)
	}
	return c.usageText
}

// CmdName returns the command name of the command.
func (c *Command) CmdName() string {
	return c.cmdName
}

// Filters returns the formal flags.
func (c *Command) Filters() map[string]*Flag {
	if c.action == nil {
		return nil
	}
	return c.action.options
}

// Args returns the command arguments.
func (c *Context) Args() []string {
	return c.args
}

// CmdPath returns the command path slice.
func (c *Context) CmdPath() []string {
	return c.cmdPath
}

// CmdPathString returns the command path string.
func (c *Context) CmdPathString() string {
	return strings.Join(c.CmdPath(), " ")
}

// ThrowStatus creates a status with stack, and panic.
func (c *Context) ThrowStatus(code int32, msg string, cause interface{}) {
	panic(status.New(code, msg, cause).TagStack(1))
}

// CheckStatus if err!=nil, create a status with stack, and panic.
// NOTE:
//  If err!=nil and msg=="", error text is set to msg
func (c *Context) CheckStatus(err error, code int32, msg string, whenError ...func()) {
	if err == nil {
		return
	}
	if len(whenError) > 0 && whenError[0] != nil {
		whenError[0]()
	}
	panic(status.New(code, msg, err).TagStack(1))
}

// String makes Author comply to the Stringer interface, to allow an easy print in the templating process
func (a Author) String() string {
	e := ""
	if a.Email != "" {
		e = " <" + a.Email + ">"
	}
	return fmt.Sprintf("%v%v", a.Name, e)
}

// defaultAppUsageTemplate is the text template for the Default help topic.
var defaultAppUsageTemplate = template.Must(template.New("appUsage").
	Funcs(template.FuncMap{"join": strings.Join}).
	Parse(`{{if .AppName}}{{.AppName}}{{else}}{{.CmdName}}{{end}}{{if .Version}} - v{{.Version}}{{end}}{{if .Description}}

{{.Description}}{{end}}

USAGE:
  {{.CmdName}}{{if .Filters}} [-globaloptions --]{{end}}{{if len .Commands}} [command] [-commandoptions]

COMMANDS:{{range .Commands}}
{{$.CmdName}} {{.UsageText}}{{end}}{{end}}{{if .Filters}}

GLOBAL OPTIONS:
{{.Filters.UsageText}}{{end}}{{if len .Authors}}

AUTHOR{{with $length := len .Authors}}{{if ne 1 $length}}S{{end}}{{end}}:
{{range $index, $author := .Authors}}{{if $index}}
{{end}}  {{$author}}{{end}}{{end}}{{if .Copyright}}

COPYRIGHT:
  {{.Copyright}}{{end}}
`))

func (c *Command) updateUsageLocked() {
	var buf bytes.Buffer
	if c.action == nil {
		return
	}
	c.action.flagSet.SetOutput(&buf)
	c.action.flagSet.PrintDefaults()
	c.usageBody = buf.String()
	if c.cmdName != "" { // non-global command
		c.usageText += fmt.Sprintf("%s # %s\n", c.cmdName, c.description)
	} else {
		c.usageBody = strings.Replace(c.usageBody, "  -", "-", -1)
		c.usageBody = strings.Replace(c.usageBody, "\n    \t", "\n  \t", -1)
	}
	c.usageText += c.usageBody
	c.action.flagSet.SetOutput(ioutil.Discard)
}

func (a *App) updateUsageLocked() {
	// if a.usageTemplate == nil {
	// 	a.usageText = ""
	// 	return
	// }
	// var data = map[string]interface{}{
	// 	"AppName":     a.appName,
	// 	"CmdName":     a.cmdName,
	// 	"Version":     a.version,
	// 	"Description": a.description,
	// 	"Authors":     a.authors,
	// 	"Commands":    []*Action{},
	// 	"Copyright":   a.copyright,
	// }
	// if len(a.actions) > 0 {
	// 	nameList := make([]string, 0, len(a.actions))
	// 	a.sortedSubCommands = make([]*Action, 0, len(a.actions))
	// 	for name := range a.actions {
	// 		nameList = append(nameList, name)
	// 	}
	// 	sort.Strings(nameList)
	// 	if nameList[0] == "" {
	// 		g := a.actions[nameList[0]]
	// 		if len(g.Filters()) > 0 {
	// 			data["Filters"] = g
	// 		}
	// 		nameList = nameList[1:]
	// 		a.sortedSubCommands = append(a.sortedSubCommands, g)
	// 	}
	// 	if len(nameList) > 0 {
	// 		actions := make([]*Action, 0, len(nameList))
	// 		for _, name := range nameList {
	// 			g := a.actions[name]
	// 			actions = append(actions, g)
	// 			a.sortedSubCommands = append(a.sortedSubCommands, g)
	// 		}
	// 		data["Commands"] = actions
	// 	}
	// }
	//
	// var buf bytes.Buffer
	// err := a.usageTemplate.Execute(&buf, data)
	// if err != nil {
	// 	panic(err)
	// }
	// a.usageText = strings.Replace(buf.String(), "\n\n\n", "\n\n", -1)
}
