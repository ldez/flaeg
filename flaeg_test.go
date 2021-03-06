package flaeg

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/containous/flaeg/parse"
	"github.com/ogier/pflag"
)

// ConfigurationWithRepeatedStruct is struct which contains repeated struct
type ConfigurationWithRepeatedStruct struct {
	Repeated  *RepeatedStruct          `description:"Repeated struct"`
	Container *RepeatedStructContainer `description:"Container"`
}

type RepeatedStruct struct {
	Val string `description:"Value"`
}

type RepeatedStructContainer struct {
	Repeated *RepeatedStruct `description:"Repeated struct"`
}

// Configuration is a struct which contains all different type to field
// using parsers on string, time.Duration, pointer, bool, int, int64, time.Time, float64
type Configuration struct {
	Name     string         // no description struct tag, it will not be flaged
	LogLevel string         `short:"l" description:"Log level"`      // string type field, short flag "-l"
	Timeout  parse.Duration `description:"Timeout duration"`         // Duration type field
	Db       *DatabaseInfo  `description:"Enable database"`          // pointer type field (on DatabaseInfo)
	Owner    *OwnerInfo     `description:"Enable Owner description"` // another pointer type field (on OwnerInfo)
}

type ServerInfo struct {
	Watch  bool   `description:"Watch device"`      // bool type
	IP     string `description:"Server ip address"` // string type field
	Load   int    `description:"Server load"`       // int type field
	Load64 int64  `description:"Server load"`       // int64 type field, same description just to be sure it works
}
type DatabaseInfo struct {
	ServerInfo             // Go through anonymous field
	ConnectionMax   uint   `long:"comax" description:"Number max of connections on database"` // uint type field, long flag "--comax"
	ConnectionMax64 uint64 `description:"Number max of connections on database"`              // uint64 type field, same description just to be sure it works
}
type OwnerInfo struct {
	Name        *string      `description:"Owner name"`                     // pointer type field on string
	DateOfBirth time.Time    `long:"dob" description:"Owner date of birth"` // time.Time type field, long flag "--dob"
	Rate        float64      `description:"Owner rate"`                     // float64 type field
	Servers     []ServerInfo `description:"Owner Server"`                   // slice of ServerInfo type field, need a custom parser
}

// newDefaultConfiguration returns a pointer on Configuration with default values
func newDefaultPointersConfiguration() *Configuration {
	var db DatabaseInfo
	db.Watch = true
	db.IP = "192.168.1.2"
	db.Load = 32
	db.Load64 = 64
	db.ConnectionMax = 3200000000            // max 4294967295
	db.ConnectionMax64 = 6400000000000000000 // max 18446744073709551615

	var own OwnerInfo
	str := "DefaultOwnerNamePointer"
	own.Name = &str
	own.DateOfBirth, _ = time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	own.Rate = 0.111
	own.Servers = []ServerInfo{
		{IP: "192.168.1.2"},
		{IP: "192.168.1.3"},
		{IP: "192.168.1.4"},
	}

	return &Configuration{
		Db:    &db,
		Owner: &own,
	}
}

// newConfiguration returns a pointer on Configuration initialized
func newConfiguration() *Configuration {
	var own OwnerInfo
	str := "InitOwnerNamePointer"
	own.Name = &str
	own.DateOfBirth, _ = time.Parse(time.RFC3339, "1993-09-12T07:32:00Z")
	own.Rate = 0.999
	return &Configuration{
		Name:     "initName",
		LogLevel: "DEBUG",
		Timeout:  parse.Duration(time.Second),
		Owner:    &own,
	}
}

func TestGetTypesRecursive(t *testing.T) {
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// Check only type
	checkType := map[string]reflect.Type{
		"loglevel":           reflect.TypeOf(""),
		"timeout":            reflect.TypeOf(parse.Duration(time.Second)),
		"db":                 reflect.TypeOf(true),
		"db.watch":           reflect.TypeOf(true),
		"db.ip":              reflect.TypeOf(""),
		"db.load":            reflect.TypeOf(0),
		"db.load64":          reflect.TypeOf(int64(0)),
		"db.comax":           reflect.TypeOf(uint(0)),
		"db.connectionmax64": reflect.TypeOf(uint64(0)),
		"owner":              reflect.TypeOf(true),
		"owner.name":         reflect.TypeOf(true),
		"owner.dob":          reflect.TypeOf(time.Now()),
		"owner.rate":         reflect.TypeOf(float64(1.1)),
		"owner.servers":      reflect.TypeOf([]ServerInfo{}),
	}

	if len(checkType) != len(flagMap) {
		t.Fatalf("expected %d elements in flagMap got %d", len(checkType), len(flagMap))
	}

	for name, field := range flagMap {
		if checkType[name] != field.Type {
			t.Errorf("Tag : %s, got %s expected %s\n", name, field.Type, checkType[name])
		}
	}
}

func TestGetFlags(t *testing.T) {
	config := newConfiguration()
	flags, err := GetFlags(config)
	if err != nil {
		t.Fatal(err)
	}

	check := []string{
		"loglevel",
		"timeout",
		"db",
		"db.watch",
		"db.ip",
		"db.load",
		"db.load64",
		"db.comax",
		"db.connectionmax64",
		"owner",
		"owner.name",
		"owner.dob",
		"owner.rate",
		"owner.servers",
	}

	if len(check) != len(flags) {
		t.Errorf("expected %d elements in parsers got %d", len(check), len(flags))
	}

	sort.Strings(check)
	sort.Strings(flags)

	if !reflect.DeepEqual(flags, check) {
		t.Errorf("Got %s expected %s\n", flags, check)
	}
}

func TestGetBoolFlags(t *testing.T) {
	config := newConfiguration()
	flags, err := GetBoolFlags(config)
	if err != nil {
		t.Fatal(err)
	}

	check := []string{
		"db",
		"db.watch",
		"owner",
		"owner.name",
	}

	if len(check) != len(flags) {
		t.Errorf("expected %d elements in parsers got %d", len(check), len(flags))
	}

	sort.Strings(check)
	sort.Strings(flags)

	if !reflect.DeepEqual(flags, check) {
		t.Errorf("Got %s expected %s\n", flags, check)
	}
}

// CUSTOM PARSER
// -- sliceServerValue format {IP,DC}
type sliceServerValue []ServerInfo

func (c *sliceServerValue) Set(s string) error {
	// could use RegExp
	srv := ServerInfo{IP: s}
	*c = append(*c, srv)
	return nil
}

func (c *sliceServerValue) Get() interface{} { return []ServerInfo(*c) }

func (c *sliceServerValue) String() string { return fmt.Sprintf("%v", *c) }

func (c *sliceServerValue) SetValue(val interface{}) {
	*c = sliceServerValue(val.([]ServerInfo))
}

func TestLoadParsers(t *testing.T) {
	// init customParsers
	customParsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}

	// test
	parsers, err := parse.LoadParsers(customParsers)
	if err != nil {
		t.Fatal(err)
	}

	// Check
	check := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	check[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	check[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	check[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	check[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	check[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	check[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	check[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	check[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	check[reflect.TypeOf(time.Now())] = &timeParser

	if len(check) != len(parsers) {
		t.Errorf("expected %d elements in parsers got %d", len(check), len(parsers))
	}

	for typ, parser := range parsers {
		if !reflect.DeepEqual(parser, check[typ]) {
			t.Errorf("Got %s expected %s\n", parser, check[typ])
		}
	}
}

// Test ParseArgs with trivial flags (ie not short, not on custom parser, not on pointer)
func TestParseArgsTrivialFlags(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--loglevel=OFF",
		"--timeout=9ms",
	}

	// test
	valMap, err := parseArgs(args, flagMap, parsers)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := map[string]parse.Parser{}
	stringParser.SetValue("OFF")
	check["loglevel"] = &stringParser
	durationParser.SetValue(parse.Duration(9 * time.Millisecond))
	check["timeout"] = &durationParser

	if len(check) != len(valMap) {
		t.Errorf("expected %d elements in valMap got %d", len(check), len(valMap))
	}

	for flag, parser := range valMap {
		if !reflect.DeepEqual(parser, check[flag]) {
			t.Errorf("Got %s expected %s\n", parser, check[flag])
		}
	}
}

// Test ParseArgs with short flags
func TestParseArgsShortFlags(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"-lWARN",
	}

	// test
	valMap, err := parseArgs(args, flagMap, parsers)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := map[string]parse.Parser{}
	err = stringParser.Set("WARN")
	if err != nil {
		t.Fatal(err)
	}

	check["loglevel"] = &stringParser

	if len(check) != len(valMap) {
		t.Errorf("expected %d elements in valMap got %d", len(check), len(valMap))
	}

	for flag, parser := range valMap {
		if !reflect.DeepEqual(parser, check[flag]) {
			t.Errorf("Got %s expected %s\n", parser, check[flag])
		}
	}
}

// Test ParseArgs call Flag on pointers
func TestParseArgsPointerFlag(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--db",
		"--owner",
	}

	// test
	valmap, err := parseArgs(args, flagMap, parsers)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := map[string]parse.Parser{}
	checkDb := parse.BoolValue(true)
	check["db"] = &checkDb
	checkOwner := parse.BoolValue(true)
	check["owner"] = &checkOwner

	if len(check) != len(valmap) {
		t.Errorf("expected %d elements in valmap got %d", len(check), len(valmap))
	}

	for flag, parser := range valmap {
		if !reflect.DeepEqual(parser, check[flag]) {
			t.Errorf("Got %s expected %s\n", parser, check[flag])
		}
	}
}

// Test ParseArgs with flags under a pointer and a long flag
func TestParseArgsUnderPointerFlag(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--owner.name",
		"--db.comax=5000000000",
	}

	// test
	valmap, err := parseArgs(args, flagMap, parsers)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := map[string]parse.Parser{}
	boolParser.SetValue(true)
	check["owner.name"] = &boolParser
	uintParser.SetValue(uint(5000000000))
	check["db.comax"] = &uintParser

	if len(check) != len(valmap) {
		t.Errorf("expected %d elements in valmap got %d", len(check), len(valmap))
	}

	for flag, parser := range valmap {
		if !reflect.DeepEqual(parser, check[flag]) {
			t.Errorf("Got %s expected %s\n", parser, check[flag])
		}
	}
}

// Test ParseArgs with flag on pointer and flag under a pointer together
func TestParseArgsPointerFlagUnderPointerFlag(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--db",
		"--db.watch",
		"--db.connectionmax64=900",
	}

	// test
	valMap, err := parseArgs(args, flagMap, parsers)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := map[string]parse.Parser{}
	boolParser.SetValue(true)
	check["db"] = &boolParser
	uint64Parser.SetValue(uint64(900))
	check["db.connectionmax64"] = &uint64Parser
	check["db.watch"] = &boolParser

	if len(check) != len(valMap) {
		t.Errorf("expected %d elements in valMap got %d", len(check), len(valMap))
	}

	for flag, parser := range valMap {
		if !reflect.DeepEqual(parser, check[flag]) {
			t.Errorf("Got %s expected %s\n", parser, check[flag])
		}
	}
}

// Test ParseArgs call Flag with custom parsers
func TestParseArgsCustomFlag(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--owner.servers=127.0.0.1",
		"--owner.servers=1.0.0.1",
	}

	// test
	valMap, err := parseArgs(args, flagMap, parsers)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := map[string]parse.Parser{}
	checkOwnerServers := sliceServerValue{
		ServerInfo{IP: "127.0.0.1"},
		ServerInfo{IP: "1.0.0.1"},
	}
	check["owner.servers"] = &checkOwnerServers

	if len(check) != len(valMap) {
		t.Errorf("expected %d elements in valMap got %d", len(check), len(valMap))
	}

	for flag, parser := range valMap {
		if !reflect.DeepEqual(parser, check[flag]) {
			t.Errorf("Got %s expected %s\n", parser, check[flag])
		}
	}
}

// Test ParseArgs with all flags possible with custom parsers
func TestParseArgsAll(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--loglevel=INFO",
		"--timeout=1s",
		"--db",
		"--db.watch",
		"--db.ip=192.168.0.1",
		"--db.load=-1",
		"--db.load64=-164",
		"--db.comax=2",
		"--db.connectionmax64=264",
		"--owner",
		"--owner.name",
		"--owner.dob=2016-04-20T17:39:00Z",
		"--owner.rate=0.222",
		"--owner.servers=1.0.0.1",
	}
	// test
	valMap, err := parseArgs(args, flagMap, parsers)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := map[string]parse.Parser{}
	stringParser.SetValue("INFO")
	check["loglevel"] = &stringParser
	durationParser.SetValue(parse.Duration(time.Second))
	check["timeout"] = &durationParser
	boolParser.SetValue(true)
	check["db"] = &boolParser
	check["db.watch"] = &boolParser
	checkDcIP := parse.StringValue("192.168.0.1")
	check["db.ip"] = &checkDcIP
	intParser.SetValue(-1)
	check["db.load"] = &intParser
	int64Parser.SetValue(int64(-164))
	check["db.load64"] = &int64Parser
	uintParser.SetValue(uint(2))
	check["db.comax"] = &uintParser
	uint64Parser.SetValue(uint64(264))
	check["db.connectionmax64"] = &uint64Parser
	check["owner"] = &boolParser
	check["owner.name"] = &boolParser
	_ = timeParser.Set("2016-04-20T17:39:00Z")
	check["owner.dob"] = &timeParser
	float64Parser.SetValue(0.222)
	check["owner.rate"] = &float64Parser
	checkOwnerServers := sliceServerValue{
		ServerInfo{IP: "1.0.0.1"},
	}
	check["owner.servers"] = &checkOwnerServers
	if len(check) != len(valMap) {
		t.Errorf("expected %d elements in valMap got %d", len(check), len(valMap))
	}
	for flag, parser := range valMap {
		if !reflect.DeepEqual(parser, check[flag]) {
			t.Errorf("Got %s expected %s\n", parser, check[flag])
		}
	}
}

func TestParseArgsErrorNoParser(t *testing.T) {
	// init config
	config := &Configuration{}

	// init valMap
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{"-lCONTINUE"}

	// test
	valMap, err := parseArgs(args, flagMap, parsers)

	// check
	if err != ErrParserNotFound {
		t.Errorf("Expected error '%v' got '%v'", ErrParserNotFound, err)
	}

	// check continue on error
	stringParser.SetValue("CONTINUE")
	checkLogLevel := &stringParser

	if !reflect.DeepEqual(valMap["loglevel"], checkLogLevel) {
		t.Errorf("Got %s expected %s\n", valMap["loglevel"], checkLogLevel)
	}

}

// Test getDefaultValue on a full complex struct, with anonymous field, and not nil pointers
func TestGetDefaultValueInitConfigAllDefault(t *testing.T) {
	// INIT
	defPointerConfig := newDefaultPointersConfiguration()
	config := newConfiguration()
	defaultValMap := make(map[string]reflect.Value)

	// TEST
	if err := getDefaultValue(reflect.ValueOf(config), reflect.ValueOf(defPointerConfig), defaultValMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	checkDefaultStr := "DefaultOwnerNamePointer"
	checkDefaultDob, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	checkDob, _ := time.Parse(time.RFC3339, "1993-09-12T07:32:00Z")
	checkValue := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf("DEBUG"),
		"timeout":            reflect.ValueOf(parse.Duration(time.Second)),
		"db":                 reflect.ValueOf(&DatabaseInfo{ServerInfo: ServerInfo{Watch: true, IP: "192.168.1.2", Load: 32, Load64: 64}, ConnectionMax: 3200000000, ConnectionMax64: 6400000000000000000}),
		"db.watch":           reflect.ValueOf(true),
		"db.ip":              reflect.ValueOf("192.168.1.2"),
		"db.load":            reflect.ValueOf(32),
		"db.load64":          reflect.ValueOf(int64(64)),
		"db.comax":           reflect.ValueOf(uint(3200000000)),
		"db.connectionmax64": reflect.ValueOf(uint64(6400000000000000000)),
		"owner":              reflect.ValueOf(&OwnerInfo{Name: nil, DateOfBirth: checkDefaultDob, Rate: 0.111, Servers: []ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}}),
		"owner.name":         reflect.ValueOf(&checkDefaultStr),
		"owner.dob":          reflect.ValueOf(checkDob),
		"owner.rate":         reflect.ValueOf(float64(0.999)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	if len(checkValue) != len(defaultValMap) {
		t.Errorf("expected %d elements in defaultValMap got %d", len(checkValue), len(defaultValMap))
	}

	for flag, val := range defaultValMap {
		if !reflect.DeepEqual(checkValue[flag].Interface(), val.Interface()) {
			t.Errorf("flag %s : \nexpected \t%+v \ngot \t\t%+v\n", flag, checkValue[flag], val)
		}
	}
}

// Test getDefaultValue on a full complex struct, with anonymous field, nil pointers and not initialized fields
func TestGetDefaultValueNoConfigNoDefault(t *testing.T) {
	config := &Configuration{}
	defPointerConfig := &Configuration{}
	defaultValMap := make(map[string]reflect.Value)

	if err := getDefaultValue(reflect.ValueOf(config), reflect.ValueOf(defPointerConfig), defaultValMap, ""); err != nil {
		t.Fatal(err)
	}

	checkValue := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf(""),
		"timeout":            reflect.ValueOf(parse.Duration(0)),
		"db":                 reflect.ValueOf(&DatabaseInfo{}),
		"db.watch":           reflect.ValueOf(false),
		"db.ip":              reflect.ValueOf(""),
		"db.load":            reflect.ValueOf(0),
		"db.load64":          reflect.ValueOf(int64(0)),
		"db.comax":           reflect.ValueOf(uint(0)),
		"db.connectionmax64": reflect.ValueOf(uint64(0)),
		"owner":              reflect.ValueOf(&OwnerInfo{}),
		"owner.name":         reflect.ValueOf(new(string)),
		"owner.dob":          reflect.ValueOf(time.Time{}),
		"owner.rate":         reflect.ValueOf(float64(0)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	if len(checkValue) != len(defaultValMap) {
		t.Errorf("expected %d elements in defaultValMap got %d", len(checkValue), len(defaultValMap))
	}

	for flag, val := range defaultValMap {
		if !reflect.DeepEqual(checkValue[flag].Interface(), val.Interface()) {
			t.Errorf("flag %s : \nexpected \t%+v \ngot \t\t%+v\n", flag, checkValue[flag], val)
		}
	}
}

// Test getDefaultValue on a full complex struct, with anonymous field, nil pointers and not initialized fields
func TestGetDefaultValueInitConfigNoDefault(t *testing.T) {
	config := &Configuration{
		Name: "defaultName", // useless field not flaged
		// LogLevel is not initialized, default value will be go default value : ""
		Timeout: parse.Duration(time.Millisecond),
	}
	defPointerConfig := &Configuration{
		Db: nil, // If pointer field is nil, default value will be go default value
		// Owner is not initialized, default value will be go default value
	}
	defaultValMap := make(map[string]reflect.Value)

	if err := getDefaultValue(reflect.ValueOf(config), reflect.ValueOf(defPointerConfig), defaultValMap, ""); err != nil {
		t.Fatal(err)
	}

	checkValue := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf(""),
		"timeout":            reflect.ValueOf(parse.Duration(time.Millisecond)),
		"db":                 reflect.ValueOf(&DatabaseInfo{}),
		"db.watch":           reflect.ValueOf(false),
		"db.ip":              reflect.ValueOf(""),
		"db.load":            reflect.ValueOf(0),
		"db.load64":          reflect.ValueOf(int64(0)),
		"db.comax":           reflect.ValueOf(uint(0)),
		"db.connectionmax64": reflect.ValueOf(uint64(0)),
		"owner":              reflect.ValueOf(&OwnerInfo{}),
		"owner.name":         reflect.ValueOf(new(string)),
		"owner.dob":          reflect.ValueOf(time.Time{}),
		"owner.rate":         reflect.ValueOf(float64(0)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	if len(checkValue) != len(defaultValMap) {
		t.Errorf("expected %d elements in defaultValMap got %d", len(checkValue), len(defaultValMap))
	}

	for flag, val := range defaultValMap {
		if !reflect.DeepEqual(checkValue[flag].Interface(), val.Interface()) {
			t.Errorf("flag %s : \nexpected \t%+v \ngot \t\t%+v\n", flag, checkValue[flag], val)
		}
	}
}

// Test getDefaultValue on a empty config but with default values on pointers
func TestGetDefaultNoConfigAllDefault(t *testing.T) {
	config := &Configuration{}
	defPointerConfig := newDefaultPointersConfiguration()
	defaultValMap := make(map[string]reflect.Value)

	if err := getDefaultValue(reflect.ValueOf(config), reflect.ValueOf(defPointerConfig), defaultValMap, ""); err != nil {
		t.Fatal(err)
	}

	checkStr := "DefaultOwnerNamePointer"
	checkDob, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	checkValue := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf(""),
		"timeout":            reflect.ValueOf(parse.Duration(0)),
		"db":                 reflect.ValueOf(&DatabaseInfo{ServerInfo: ServerInfo{Watch: true, IP: "192.168.1.2", Load: 32, Load64: 64}, ConnectionMax: 3200000000, ConnectionMax64: 6400000000000000000}),
		"db.watch":           reflect.ValueOf(true),
		"db.ip":              reflect.ValueOf("192.168.1.2"),
		"db.load":            reflect.ValueOf(32),
		"db.load64":          reflect.ValueOf(int64(64)),
		"db.comax":           reflect.ValueOf(uint(3200000000)),
		"db.connectionmax64": reflect.ValueOf(uint64(6400000000000000000)),
		"owner":              reflect.ValueOf(&OwnerInfo{Name: nil, DateOfBirth: checkDob, Rate: 0.111, Servers: []ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}}),
		"owner.name":         reflect.ValueOf(&checkStr),
		"owner.dob":          reflect.ValueOf(checkDob),
		"owner.rate":         reflect.ValueOf(float64(0.111)),
		"owner.servers":      reflect.ValueOf([]ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}),
	}

	for flag, val := range defaultValMap {
		if !reflect.DeepEqual(checkValue[flag].Interface(), val.Interface()) {
			t.Errorf("flag %s : \nexpected \t%+v \ngot \t\t%+v\n", flag, checkValue[flag], val)
		}
	}
}

// Test fillStructRecursive on empty config with trivial valMap field and without default values on pointers
func TestFillStructRecursiveNoConfigNoDefaultTrivialValMap(t *testing.T) {
	config := &Configuration{}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init valMap
	valMap := map[string]parse.Parser{}
	stringParser.SetValue("INFO")
	valMap["loglevel"] = &stringParser
	durationParser.SetValue(parse.Duration(time.Second))
	valMap["timeout"] = &durationParser

	// init defaultValMap NoConfigNoDefault
	defaultValMap := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf(""),
		"timeout":            reflect.ValueOf(parse.Duration(time.Duration(0))),
		"db":                 reflect.ValueOf(&DatabaseInfo{}),
		"db.watch":           reflect.ValueOf(false),
		"db.ip":              reflect.ValueOf(""),
		"db.load":            reflect.ValueOf(0),
		"db.load64":          reflect.ValueOf(int64(0)),
		"db.comax":           reflect.ValueOf(uint(0)),
		"db.connectionmax64": reflect.ValueOf(uint64(0)),
		"owner":              reflect.ValueOf(&OwnerInfo{}),
		"owner.name":         reflect.ValueOf(new(string)),
		"owner.dob":          reflect.ValueOf(time.Time{}),
		"owner.rate":         reflect.ValueOf(float64(0)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	// test
	if err := fillStructRecursive(reflect.ValueOf(config), defaultValMap, valMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := &Configuration{}
	check.LogLevel = "INFO"
	check.Timeout = parse.Duration(time.Second)
	if !reflect.DeepEqual(config, check) {
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test fillStructRecursive on empty config with all valmap field but without default values on pointers
func TestFillStructRecursiveNoConfigNoDefaultAllValmap(t *testing.T) {
	config := &Configuration{}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init valMap
	valMap := map[string]parse.Parser{}
	stringParser.SetValue("INFO")
	valMap["loglevel"] = &stringParser
	durationParser.SetValue(parse.Duration(time.Second))
	valMap["timeout"] = &durationParser
	boolParser.SetValue(true)
	valMap["db"] = &boolParser
	valMap["db.watch"] = &boolParser
	valmapDcIP := parse.StringValue("192.168.0.1")
	valMap["db.ip"] = &valmapDcIP
	intParser.SetValue(-1)
	valMap["db.load"] = &intParser
	int64Parser.SetValue(int64(-164))
	valMap["db.load64"] = &int64Parser
	uintParser.SetValue(uint(2))
	valMap["db.comax"] = &uintParser
	uint64Parser.SetValue(uint64(264))
	valMap["db.connectionmax64"] = &uint64Parser
	valMap["owner"] = &boolParser
	valMap["owner.name"] = &boolParser
	_ = timeParser.Set("2016-04-20T17:39:00Z")
	valMap["owner.dob"] = &timeParser
	float64Parser.SetValue(0.222)
	valMap["owner.rate"] = &float64Parser
	valmapOwnerServers := sliceServerValue{
		ServerInfo{IP: "1.0.0.1"},
	}
	valMap["owner.servers"] = &valmapOwnerServers

	// init defaultValMap NoConfigNoDefault
	defaultValMap := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf(""),
		"timeout":            reflect.ValueOf(parse.Duration(0)),
		"db":                 reflect.ValueOf(&DatabaseInfo{}),
		"db.watch":           reflect.ValueOf(false),
		"db.ip":              reflect.ValueOf(""),
		"db.load":            reflect.ValueOf(0),
		"db.load64":          reflect.ValueOf(int64(0)),
		"db.comax":           reflect.ValueOf(uint(0)),
		"db.connectionmax64": reflect.ValueOf(uint64(0)),
		"owner":              reflect.ValueOf(&OwnerInfo{}),
		"owner.name":         reflect.ValueOf(new(string)),
		"owner.dob":          reflect.ValueOf(time.Time{}),
		"owner.rate":         reflect.ValueOf(float64(0)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	// test
	if err := fillStructRecursive(reflect.ValueOf(config), defaultValMap, valMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	checkDob, _ := time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")
	check := &Configuration{
		LogLevel: "INFO",
		Timeout:  parse.Duration(time.Second),
		Db: &DatabaseInfo{
			ServerInfo: ServerInfo{
				Watch:  true,
				IP:     "192.168.0.1",
				Load:   -1,
				Load64: int64(-164),
			},
			ConnectionMax:   uint(2),
			ConnectionMax64: uint64(264),
		},
		Owner: &OwnerInfo{
			Name:        new(string),
			DateOfBirth: checkDob,
			Rate:        float64(0.222),
			Servers:     []ServerInfo{{IP: "1.0.0.1"}},
		},
	}

	if !reflect.DeepEqual(config, check) {
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test fillStructRecursive on empty config without valmap and without default values pointers
func TestFillStructRecursiveNoConfigAllDefaultNoValmap(t *testing.T) {
	config := &Configuration{}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init valMap
	valMap := map[string]parse.Parser{}

	// init defaultValMap NoConfigAllDefault
	defaultStr := "DefaultOwnerNamePointer"
	defaultDob, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	defaultValMap := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf(""),
		"timeout":            reflect.ValueOf(parse.Duration(0)),
		"db":                 reflect.ValueOf(&DatabaseInfo{ServerInfo: ServerInfo{Watch: true, IP: "192.168.1.2", Load: 32, Load64: 64}, ConnectionMax: 3200000000, ConnectionMax64: 6400000000000000000}),
		"db.watch":           reflect.ValueOf(true),
		"db.ip":              reflect.ValueOf("192.168.1.2"),
		"db.load":            reflect.ValueOf(32),
		"db.load64":          reflect.ValueOf(int64(64)),
		"db.comax":           reflect.ValueOf(uint(3200000000)),
		"db.connectionmax64": reflect.ValueOf(uint64(6400000000000000000)),
		"owner":              reflect.ValueOf(&OwnerInfo{Name: nil, DateOfBirth: defaultDob, Rate: 0.111, Servers: []ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}}),
		"owner.name":         reflect.ValueOf(&defaultStr),
		"owner.dob":          reflect.ValueOf(defaultDob),
		"owner.rate":         reflect.ValueOf(float64(0.111)),
		"owner.servers":      reflect.ValueOf([]ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}),
	}
	// test
	if err := fillStructRecursive(reflect.ValueOf(config), defaultValMap, valMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := &Configuration{}
	if !reflect.DeepEqual(config, check) {
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test fillStructRecursive on not-empty config with default values on pointers but without valmap field
func TestFillStructRecursiveInitConfigAllDefaultNoValmap(t *testing.T) {
	config := newConfiguration()

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init valMap
	valMap := map[string]parse.Parser{}

	// init defaultValMap InitConfigAllDefault from TestGetDefaultValueAll
	checkDefaultStr := "DefaultOwnerNamePointer"
	checkDefaultDob, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	checkDob, _ := time.Parse(time.RFC3339, "1993-09-12T07:32:00Z")
	defaultValMap := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf("DEBUG"),
		"timeout":            reflect.ValueOf(parse.Duration(time.Second)),
		"db":                 reflect.ValueOf(&DatabaseInfo{ServerInfo: ServerInfo{Watch: true, IP: "192.168.1.2", Load: 32, Load64: 64}, ConnectionMax: 3200000000, ConnectionMax64: 6400000000000000000}),
		"db.watch":           reflect.ValueOf(true),
		"db.ip":              reflect.ValueOf("192.168.1.2"),
		"db.load":            reflect.ValueOf(32),
		"db.load64":          reflect.ValueOf(int64(64)),
		"db.comax":           reflect.ValueOf(uint(3200000000)),
		"db.connectionmax64": reflect.ValueOf(uint64(6400000000000000000)),
		"owner":              reflect.ValueOf(&OwnerInfo{Name: nil, DateOfBirth: checkDefaultDob, Rate: 0.111, Servers: []ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}}),
		"owner.name":         reflect.ValueOf(&checkDefaultStr),
		"owner.dob":          reflect.ValueOf(checkDob),
		"owner.rate":         reflect.ValueOf(float64(0.999)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	// test
	if err := fillStructRecursive(reflect.ValueOf(config), defaultValMap, valMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	// fmt.Printf("Got : %+v\n", config)
	check := newConfiguration()

	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Owner, check.Owner) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Owner, config.Owner)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test fillStructRecursive on empty config with only pointer valmap field and with default values on pointers
func TestFillStructRecursiveInitConfigAllDefaultPointerValmap(t *testing.T) {
	config := &Configuration{}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init valMap
	valMap := map[string]parse.Parser{}
	boolParser.SetValue(true)
	valMap["db"] = &boolParser
	valMap["owner"] = &boolParser

	// init defaultValMap InitConfigAllDefault from TestGetDefaultValueAll
	checkDefaultStr := "DefaultOwnerNamePointer"
	checkDefaultDob, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	checkDob, _ := time.Parse(time.RFC3339, "1993-09-12T07:32:00Z")
	defaultValMap := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf("DEBUG"),
		"timeout":            reflect.ValueOf(parse.Duration(time.Second)),
		"db":                 reflect.ValueOf(&DatabaseInfo{ServerInfo: ServerInfo{Watch: true, IP: "192.168.1.2", Load: 32, Load64: 64}, ConnectionMax: 3200000000, ConnectionMax64: 6400000000000000000}),
		"db.watch":           reflect.ValueOf(true),
		"db.ip":              reflect.ValueOf("192.168.1.2"),
		"db.load":            reflect.ValueOf(32),
		"db.load64":          reflect.ValueOf(int64(64)),
		"db.comax":           reflect.ValueOf(uint(3200000000)),
		"db.connectionmax64": reflect.ValueOf(uint64(6400000000000000000)),
		"owner":              reflect.ValueOf(&OwnerInfo{Name: nil, DateOfBirth: checkDefaultDob, Rate: 0.111, Servers: []ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}}),
		"owner.name":         reflect.ValueOf(&checkDefaultStr),
		"owner.dob":          reflect.ValueOf(checkDob),
		"owner.rate":         reflect.ValueOf(float64(0.999)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	// test
	if err := fillStructRecursive(reflect.ValueOf(config), defaultValMap, valMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := &Configuration{
		Db: &DatabaseInfo{
			ServerInfo:      ServerInfo{true, "192.168.1.2", 32, 64},
			ConnectionMax:   3200000000,
			ConnectionMax64: 6400000000000000000,
		},
		Owner: &OwnerInfo{
			Name:        nil,
			DateOfBirth: checkDefaultDob,
			Rate:        0.111,
			Servers: []ServerInfo{
				{IP: "192.168.1.2"},
				{IP: "192.168.1.3"},
				{IP: "192.168.1.4"},
			},
		},
	}

	if !reflect.DeepEqual(config, check) {
		fmt.Printf("expected\t: %+v\ngot\t\t\t: %+v\n", check.Db, config.Db)
		fmt.Printf("expected\t: %+v\ngot\t\t\t: %+v\n", check.Owner, config.Owner)

		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test fillStructRecursive on not-empty struc with only one pointer under pointer valmap field and with default values on pointers
func TestFillStructRecursiveInitConfigAllDefaultPointerUnderPointerValmap(t *testing.T) {
	config := newConfiguration()

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init valMap
	valMap := map[string]parse.Parser{}
	boolParser.SetValue(true)
	valMap["owner.name"] = &boolParser

	// init defaultValMap InitConfigAllDefault from TestGetDefaultValueAll
	checkDefaultStr := "DefaultOwnerNamePointer"
	checkDefaultDob, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	checkDob, _ := time.Parse(time.RFC3339, "1993-09-12T07:32:00Z")
	defaultValMap := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf("DEBUG"),
		"timeout":            reflect.ValueOf(parse.Duration(time.Second)),
		"db":                 reflect.ValueOf(&DatabaseInfo{ServerInfo: ServerInfo{Watch: true, IP: "192.168.1.2", Load: 32, Load64: 64}, ConnectionMax: 3200000000, ConnectionMax64: 6400000000000000000}),
		"db.watch":           reflect.ValueOf(true),
		"db.ip":              reflect.ValueOf("192.168.1.2"),
		"db.load":            reflect.ValueOf(32),
		"db.load64":          reflect.ValueOf(int64(64)),
		"db.comax":           reflect.ValueOf(uint(3200000000)),
		"db.connectionmax64": reflect.ValueOf(uint64(6400000000000000000)),
		"owner":              reflect.ValueOf(&OwnerInfo{Name: nil, DateOfBirth: checkDefaultDob, Rate: 0.111, Servers: []ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}}),
		"owner.name":         reflect.ValueOf(&checkDefaultStr),
		"owner.dob":          reflect.ValueOf(checkDob),
		"owner.rate":         reflect.ValueOf(float64(0.999)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	// test
	if err := fillStructRecursive(reflect.ValueOf(config), defaultValMap, valMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := &Configuration{
		Name:     "initName",
		LogLevel: "DEBUG",
		Timeout:  parse.Duration(time.Second),
		Owner: &OwnerInfo{
			Name:        &checkDefaultStr,
			DateOfBirth: checkDob,
			Rate:        0.999,
		},
	}

	if !reflect.DeepEqual(config, check) {
		fmt.Printf("expected\t: %+v\ngot\t\t\t: %+v\n", check.Db, config.Db)
		fmt.Printf("expected\t: %+v\ngot\t\t\t: %+v\n", check.Owner, config.Owner)
		fmt.Printf("expected\t: %+v\tgot\t\t\t: %+v\n", *check.Owner.Name, *config.Owner.Name)
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test fillStructRecursive on empty config with some random valmap field and with default values on pointers
func TestFillStructRecursiveNoConfigAllDefaultSomeValmap(t *testing.T) {
	config := &Configuration{}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init valMap
	valMap := map[string]parse.Parser{}
	durationParser.SetValue(5 * parse.Duration(time.Second))
	valMap["timeout"] = &durationParser
	boolParser.SetValue(true)
	valMap["db"] = &boolParser
	_ = timeParser.Set("2016-04-20T17:39:00Z")
	valMap["owner.dob"] = &timeParser
	float64Parser.SetValue(0.222)
	valMap["owner.rate"] = &float64Parser

	// init defaultValMap NoConfigAllDefault
	defaultStr := "DefaultOwnerNamePointer"
	defaultDob, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	defaultValMap := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf(""),
		"timeout":            reflect.ValueOf(parse.Duration(0)),
		"db":                 reflect.ValueOf(&DatabaseInfo{ServerInfo: ServerInfo{Watch: true, IP: "192.168.1.2", Load: 32, Load64: 64}, ConnectionMax: 3200000000, ConnectionMax64: 6400000000000000000}),
		"db.watch":           reflect.ValueOf(true),
		"db.ip":              reflect.ValueOf("192.168.1.2"),
		"db.load":            reflect.ValueOf(32),
		"db.load64":          reflect.ValueOf(int64(64)),
		"db.comax":           reflect.ValueOf(uint(3200000000)),
		"db.connectionmax64": reflect.ValueOf(uint64(6400000000000000000)),
		"owner":              reflect.ValueOf(&OwnerInfo{Name: nil, DateOfBirth: defaultDob, Rate: 0.111, Servers: []ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}}),
		"owner.name":         reflect.ValueOf(&defaultStr),
		"owner.dob":          reflect.ValueOf(defaultDob),
		"owner.rate":         reflect.ValueOf(float64(0.111)),
		"owner.servers":      reflect.ValueOf([]ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}),
	}
	// test
	if err := fillStructRecursive(reflect.ValueOf(config), defaultValMap, valMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := &Configuration{}
	check.Timeout = 5 * parse.Duration(time.Second)
	check.Db = newDefaultPointersConfiguration().Db
	check.Owner = newDefaultPointersConfiguration().Owner
	check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")
	check.Owner.Rate = 0.222
	check.Owner.Name = nil
	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Owner, check.Owner) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Owner, config.Owner)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test fillStructRecursive with repeated struct (with one on root)
func TestFillStructRecursiveWithRepeatedPtrStruct(t *testing.T) {
	config := &ConfigurationWithRepeatedStruct{}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{}
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser

	// init valMap
	valMap := map[string]parse.Parser{}
	stringParser.SetValue("test")
	valMap["container.repeated.val"] = &stringParser

	// init defaultValMap NoConfigAllDefault
	defaultValMap := map[string]reflect.Value{
		"repeated":  reflect.ValueOf(&RepeatedStruct{Val: "Default"}),
		"container": reflect.ValueOf(&RepeatedStructContainer{&RepeatedStruct{Val: "DefaultInContainer"}}),
	}

	// test
	if err := fillStructRecursive(reflect.ValueOf(config), defaultValMap, valMap, ""); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := &ConfigurationWithRepeatedStruct{}
	check.Container = &RepeatedStructContainer{
		&RepeatedStruct{
			Val: "test",
		},
	}
	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Container, check.Container) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Container, config.Container)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test LoadWithParsers on not empty config without default values on pointers and without flag called
func TestLoadWithParsersInitConfigNoDefaultNoFlag(t *testing.T) {
	// INIT
	config := newConfiguration()
	defaultPointers := &Configuration{}

	customParsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}

	var args []string

	// TEST
	if err := LoadWithParsers(config, defaultPointers, args, customParsers); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := newConfiguration()
	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Owner, check.Owner) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Owner, config.Owner)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test LoadWithParsers on not empty config with all default values on pointers and without flag called
func TestLoadWithParsersInitConfigAllDefaultNoFlag(t *testing.T) {
	// INIT
	config := newConfiguration()
	defaultPointers := newDefaultPointersConfiguration()

	customParsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}

	var args []string

	// TEST
	if err := LoadWithParsers(config, defaultPointers, args, customParsers); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := newConfiguration()
	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Owner, check.Owner) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Owner, config.Owner)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test LoadWithParsers on not empty config without default values on pointers and with all flags called
func TestLoadWithParsersInitConfigNoDefaultAllFlag(t *testing.T) {
	// INIT
	config := newConfiguration()
	defaultPointers := &Configuration{}

	customParsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}

	args := []string{
		"--loglevel=INFO",
		"--timeout=1s",
		"--db",
		"--db.watch",
		"--db.ip=192.168.0.1",
		"--db.load=-1",
		"--db.load64=-164",
		"--db.comax=2",
		"--db.connectionmax64=264",
		"--owner",
		"--owner.name",
		"--owner.dob=2016-04-20T17:39:00Z",
		"--owner.rate=0.222",
		"--owner.servers=1.0.0.1",
	}

	// TEST
	if err := LoadWithParsers(config, defaultPointers, args, customParsers); err != nil {
		t.Fatal(err)
	}

	// CHECK
	checkDob, _ := time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")
	check := &Configuration{
		Name:     "initName",
		LogLevel: "INFO",
		Timeout:  parse.Duration(time.Second),
		Db: &DatabaseInfo{
			ServerInfo: ServerInfo{
				Watch:  true,
				IP:     "192.168.0.1",
				Load:   -1,
				Load64: int64(-164),
			},
			ConnectionMax:   uint(2),
			ConnectionMax64: uint64(264),
		},
		Owner: &OwnerInfo{
			Name:        new(string),
			DateOfBirth: checkDob,
			Rate:        float64(0.222),
			Servers:     []ServerInfo{{IP: "1.0.0.1"}},
		},
	}

	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Owner, check.Owner) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Owner, config.Owner)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test LoadWithParsers on not empty config with all default values on pointers and with some flags called
func TestLoadWithParsersInitConfigAllDefaultSomeFlag(t *testing.T) {
	// INIT
	config := newConfiguration()
	defaultPointers := newDefaultPointersConfiguration()

	customParsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}

	args := []string{
		"--loglevel=INFO",
		"--db",
		"--owner.name",
		"--owner.dob=2016-04-20T17:39:00Z",
	}

	// TEST
	if err := LoadWithParsers(config, defaultPointers, args, customParsers); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := newConfiguration()
	check.LogLevel = "INFO"
	check.Db = newDefaultPointersConfiguration().Db
	check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
	check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Owner, check.Owner) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Owner, config.Owner)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test LoadWithParsers on empty config with all default values on pointers and with some flags called
func TestLoadWithParsersNoConfigAllDefaultSomeFlag(t *testing.T) {
	// INIT
	config := &Configuration{}
	defaultPointers := newDefaultPointersConfiguration()

	customParsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}

	args := []string{
		"--loglevel=INFO",
		"--db=FALSE",
		"--owner.dob=2016-04-20T17:39:00Z",
	}

	// TEST
	if err := LoadWithParsers(config, defaultPointers, args, customParsers); err != nil {
		t.Fatal(err)
	}

	// CHECK
	check := newDefaultPointersConfiguration()
	check.LogLevel = "INFO"
	check.Db = nil
	check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")
	check.Owner.Name = nil

	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Owner, check.Owner) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Owner, config.Owner)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test Load without parsers on not empty config with all default values on pointers and with some flags called
func TestLoadInitConfigAllDefaultSomeFlagErrorParser(t *testing.T) {
	// INIT
	config := newConfiguration()
	defaultPointers := newDefaultPointersConfiguration()

	args := []string{
		"--loglevel=INFO",
		"--db",
		"--owner.name",
		"--owner.dob=2016-04-20T17:39:00Z",
	}

	// catch stdout
	backupStdout := os.Stdout
	defer func() {
		os.Stdout = backupStdout
	}()
	_, w, _ := os.Pipe()
	os.Stdout = w

	// TEST
	err := Load(config, defaultPointers, args)
	if err != ErrParserNotFound {
		t.Errorf("Expected error %s\ngot %s", ErrParserNotFound, err)
	}

	// read and restore stdout
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = backupStdout

	// check continue on error
	check := newConfiguration()
	check.LogLevel = "INFO"
	check.Db = newDefaultPointersConfiguration().Db
	check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
	check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

	if !reflect.DeepEqual(config, check) {
		if !reflect.DeepEqual(config.Owner, check.Owner) {
			t.Errorf("\nexpected\t: %+v\ngot\t\t\t: %+v", check.Owner, config.Owner)
		}
		t.Errorf("Error :\nexpected \t%+v \ngot \t\t%+v\n", check, config)
	}
}

// Test Parse Args Error with an invalid argument
func TestParseArgsInvalidArgument(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)
	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--timeout=ItsAnError",
	}

	// Test
	checkErr := "invalid argument"
	if _, err := parseArgs(args, flagMap, parsers); err == nil || !strings.Contains(err.Error(), checkErr) {
		t.Errorf("Expected Error : invalid argument got Error : %s", err)
	}
}

// Test Parse Args Error with an unknown flag
func TestParseArgsErrorUnknownFlag(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)

	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--unknownFlag",
	}

	// Test
	if _, err := parseArgs(args, flagMap, parsers); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("Expected Error : unknown flag got Error : %s", err)
	}
}

// Test Print Error with an invalid argument
func TestPrintErrorInvalidArgument(t *testing.T) {
	// We assume that getTypesRecursive works well
	config := newConfiguration()
	flagMap := make(map[string]reflect.StructField)

	if err := getTypesRecursive(reflect.ValueOf(config), flagMap, ""); err != nil {
		t.Fatal(err)
	}

	// init parsers
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf([]ServerInfo{}): &sliceServerValue{},
	}
	var boolParser parse.BoolValue
	parsers[reflect.TypeOf(true)] = &boolParser
	var intParser parse.IntValue
	parsers[reflect.TypeOf(1)] = &intParser
	var int64Parser parse.Int64Value
	parsers[reflect.TypeOf(int64(1))] = &int64Parser
	var uintParser parse.UintValue
	parsers[reflect.TypeOf(uint(1))] = &uintParser
	var uint64Parser parse.Uint64Value
	parsers[reflect.TypeOf(uint64(1))] = &uint64Parser
	var stringParser parse.StringValue
	parsers[reflect.TypeOf("")] = &stringParser
	var float64Parser parse.Float64Value
	parsers[reflect.TypeOf(float64(1.5))] = &float64Parser
	var durationParser parse.Duration
	parsers[reflect.TypeOf(parse.Duration(time.Second))] = &durationParser
	var timeParser parse.TimeValue
	parsers[reflect.TypeOf(time.Now())] = &timeParser

	// init args
	args := []string{
		"--timeout=ItsAnError",
	}

	// init defaultValMap
	checkDefaultStr := "DefaultOwnerNamePointer"
	checkDefaultDob, _ := time.Parse(time.RFC3339, "1979-05-27T07:32:00Z")
	checkDob, _ := time.Parse(time.RFC3339, "1993-09-12T07:32:00Z")
	defaultValMap := map[string]reflect.Value{
		"loglevel":           reflect.ValueOf("DEBUG"),
		"timeout":            reflect.ValueOf(parse.Duration(time.Second)),
		"db":                 reflect.ValueOf(&DatabaseInfo{ServerInfo: ServerInfo{Watch: true, IP: "192.168.1.2", Load: 32, Load64: 64}, ConnectionMax: 3200000000, ConnectionMax64: 6400000000000000000}),
		"db.watch":           reflect.ValueOf(true),
		"db.ip":              reflect.ValueOf("192.168.1.2"),
		"db.load":            reflect.ValueOf(32),
		"db.load64":          reflect.ValueOf(int64(64)),
		"db.comax":           reflect.ValueOf(uint(3200000000)),
		"db.connectionmax64": reflect.ValueOf(uint64(6400000000000000000)),
		"owner":              reflect.ValueOf(&OwnerInfo{Name: &checkDefaultStr, DateOfBirth: checkDefaultDob, Rate: 0.111, Servers: []ServerInfo{{Watch: false, IP: "192.168.1.2", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.3", Load: 0, Load64: 0}, {Watch: false, IP: "192.168.1.4", Load: 0, Load64: 0}}}),
		"owner.name":         reflect.ValueOf(&checkDefaultStr),
		"owner.dob":          reflect.ValueOf(checkDob),
		"owner.rate":         reflect.ValueOf(float64(0.999)),
		"owner.servers":      reflect.ValueOf(*new([]ServerInfo)),
	}

	// catch stdout
	backupStdout := os.Stdout
	defer func() {
		os.Stdout = backupStdout
	}()
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Test
	checkErr := "invalid argument"
	_, err := parseArgs(args, flagMap, parsers)
	if err != nil && strings.Contains(err.Error(), checkErr) {
		_ = PrintError(err, flagMap, defaultValMap, parsers)
	} else {
		t.Errorf("Expected Error : invalid argument got Error : %s", err)
	}

	// read and restore stdout
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = backupStdout

	// check
	if !strings.Contains(string(out), checkErr) {
		t.Errorf("Expected error %s\ngot %s", checkErr, out)
	}
}

// Test Commands feature with only the root command
func TestFlaegCommandRootInitConfigAllDefaultSomeFlag(t *testing.T) {
	// INIT
	config := newConfiguration()
	defaultPointers := newDefaultPointersConfiguration()

	args := []string{
		"--loglevel=INFO",
		"--db",
		"--owner.name",
		"--owner.dob=2016-04-20T17:39:00Z",
	}

	// init command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,
		Config:                config,
		DefaultPointersConfig: defaultPointers,
		Run: func() error {
			// CHECK
			check := newConfiguration()
			check.LogLevel = "INFO"
			check.Db = newDefaultPointersConfiguration().Db
			check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
			check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

			if !reflect.DeepEqual(config, check) {
				msg := fmt.Sprintf("\nexpected \t%+v \ngot \t\t%+v\n", check, config)
				return errors.New(msg)
			}
			return nil
		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})

	// run test
	if err := flaeg.Run(); err != nil {
		t.Fatal(err)
	}
}

// Version Config
type VersionConfig struct {
	Version string `short:"v" description:"Version"`
}

// Test Commands feature with root and version commands
func TestCommandVersionInitConfigNoDefaultNoFlag(t *testing.T) {
	// INIT
	rootConfig := newConfiguration()
	rootDefaultPointers := newDefaultPointersConfiguration()
	versionConfig := &VersionConfig{"0.1"}

	args := []string{
		"version", // call Command
	}

	// root command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,
		Config:                rootConfig,
		DefaultPointersConfig: rootDefaultPointers,
		Run: func() error {
			fmt.Printf("Run with config :\n%+v\n", rootConfig)
			// CHECK
			check := newConfiguration()
			check.LogLevel = "INFO"
			check.Db = newDefaultPointersConfiguration().Db
			check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
			check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

			if !reflect.DeepEqual(rootConfig, check) {
				msg := fmt.Sprintf("\nexpected \t%+v \ngot \t\t%+v\n", check, rootConfig)
				return errors.New(msg)
			}
			return nil
		},
	}

	// version command
	versionCmd := &Command{
		Name:                  "version",
		Description:           `Print version`,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		Run: func() error {
			// CHECK
			if versionConfig.Version != "0.1" {
				return fmt.Errorf("expected 0.1 got %s", versionConfig.Version)
			}
			return nil
		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})
	// add command Version
	flaeg.AddCommand(versionCmd)

	// run test
	if err := flaeg.Run(); err != nil {
		t.Fatal(err)
	}
}

// Test Commands feature with root and version commands
func TestCommandVersionInitConfigNoDefaultAllFlag(t *testing.T) {
	// INIT
	rootConfig := newConfiguration()
	rootDefaultPointers := newDefaultPointersConfiguration()
	versionConfig := &VersionConfig{"0.1"}

	args := []string{
		"version", // call Command
		"-v2.2beta",
	}

	// root command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,
		Config:                rootConfig,
		DefaultPointersConfig: rootDefaultPointers,
		Run: func() error {
			fmt.Printf("Run with config :\n%+v\n", rootConfig)
			// CHECK
			check := newConfiguration()
			check.LogLevel = "INFO"
			check.Db = newDefaultPointersConfiguration().Db
			check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
			check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

			if !reflect.DeepEqual(rootConfig, check) {
				msg := fmt.Sprintf("\nexpected \t%+v \ngot \t\t%+v\n", check, rootConfig)
				return errors.New(msg)
			}
			return nil
		},
	}

	// version command
	versionCmd := &Command{
		Name:                  "version",
		Description:           `Print version`,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		Run: func() error {
			// CHECK
			if versionConfig.Version != "2.2beta" {
				return fmt.Errorf("expected 2.2beta got %s", versionConfig.Version)
			}
			return nil
		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})
	// add command Version
	flaeg.AddCommand(versionCmd)

	// run test
	if err := flaeg.Run(); err != nil {
		t.Fatal(err)
	}
}

// Test Commands feature with root and version commands
func TestCommandVersionInitConfigNoDefaultCommandHelpFlag(t *testing.T) {
	// INIT
	rootConfig := newConfiguration()
	rootDefaultPointers := newDefaultPointersConfiguration()
	versionConfig := &VersionConfig{"0.1"}
	args := []string{
		"version", // call Command
		"-h",
	}

	// root command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,
		Config:                rootConfig,
		DefaultPointersConfig: rootDefaultPointers,
		Run: func() error {
			fmt.Printf("Run with config :\n%+v\n", rootConfig)

			// CHECK
			check := newConfiguration()
			check.LogLevel = "INFO"
			check.Db = newDefaultPointersConfiguration().Db
			check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
			check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

			if !reflect.DeepEqual(rootConfig, check) {
				msg := fmt.Sprintf("\nexpected \t%+v \ngot \t\t%+v\n", check, rootConfig)
				return errors.New(msg)
			}
			return nil
		},
	}

	// version command
	versionCmd := &Command{
		Name:                  "version",
		Description:           `Print version`,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		Run: func() error {
			fmt.Printf("Version %s \n", versionConfig.Version)
			return nil
		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})
	// add command Version
	flaeg.AddCommand(versionCmd)

	// catch stdout
	backupStdout := os.Stdout
	defer func() {
		os.Stdout = backupStdout
	}()
	_, w, _ := os.Pipe()
	os.Stdout = w

	// run test
	if err := flaeg.Run(); err == nil || err != pflag.ErrHelp {
		t.Errorf("Expected Error :help requested got Error : %v", err)
	}

	// read and restore stdout
	err := w.Close()
	if err != nil {
		t.Fatal(err)
	}
}

// Test Commands feature with root and version commands
func TestSeveralCommandsDashArg(t *testing.T) {
	// INIT
	rootConfig := newConfiguration()
	rootDefaultPointers := newDefaultPointersConfiguration()
	versionConfig := &VersionConfig{"0.1"}
	args := []string{
		"-", // dash arg
	}

	// root command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,
		Config:                rootConfig,
		DefaultPointersConfig: rootDefaultPointers,
		Run: func() error {
			// CHECK
			check := newConfiguration()

			if !reflect.DeepEqual(rootConfig, check) {
				msg := fmt.Sprintf("\nexpected \t%+v \ngot \t\t%+v\n", check, rootConfig)
				return errors.New(msg)
			}
			return nil
		},
	}

	// version command
	versionCmd := &Command{
		Name:                  "version",
		Description:           `Print version`,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		Run: func() error {
			fmt.Printf("Version %s \n", versionConfig.Version)
			return nil
		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})
	// add command Version
	flaeg.AddCommand(versionCmd)

	// run test
	if err := flaeg.Run(); err != nil {
		t.Fatal(err)
	}
}

// Test Commands feature with root and version commands
func TestCommandVersionInitConfigNoDefaultRootCommandHelpFlag(t *testing.T) {
	// INIT
	rootConfig := newConfiguration()
	rootDefaultPointers := newDefaultPointersConfiguration()
	versionConfig := &VersionConfig{"0.1"}
	args := []string{
		"--help",
	}

	// root command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,
		Config:                rootConfig,
		DefaultPointersConfig: rootDefaultPointers,
		Run: func() error {
			fmt.Printf("Run with config :\n%+v\n", rootConfig)
			// CHECK
			check := newConfiguration()
			check.LogLevel = "INFO"
			check.Db = newDefaultPointersConfiguration().Db
			check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
			check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

			if !reflect.DeepEqual(rootConfig, check) {
				msg := fmt.Sprintf("\nexpected \t%+v \ngot \t\t%+v\n", check, rootConfig)
				return errors.New(msg)
			}
			return nil
		},
	}

	// version command
	versionCmd := &Command{
		Name:                  "version",
		Description:           `Print version`,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		Run: func() error {
			fmt.Printf("Version %s \n", versionConfig.Version)
			return nil
		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})
	// add command Version
	flaeg.AddCommand(versionCmd)

	// catch stdout
	backupStdout := os.Stdout
	defer func() {
		os.Stdout = backupStdout
	}()
	r, w, _ := os.Pipe()
	os.Stdout = w

	// run test
	if err := flaeg.Run(); err == nil || err != pflag.ErrHelp {
		t.Errorf("Expected Error :help requested got Error : %v", err)
	}

	// read and restore stdout
	err := w.Close()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = backupStdout

	if !strings.Contains(string(out), "flaegtest is a test program made to test flaeg library.") {
		t.Errorf("Expected root command help")
	}
}

func TestCommandHidden(t *testing.T) {
	// INIT
	args := []string{
		"--help",
	}

	// hidden root command
	hiddenDescription := "The Hidden root command"
	versionConfig := &VersionConfig{Version: "0.1"}
	rootCmd := &Command{
		Name:                  "secret",
		Description:           hiddenDescription,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		HideHelp:              true,
		Run: func() error {
			return nil
		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})

	// catch stdout
	backupStdout := os.Stdout
	defer func() {
		os.Stdout = backupStdout
	}()
	r, w, _ := os.Pipe()
	os.Stdout = w

	// run test
	expectedErrMessage := "command secret not found"
	if err := flaeg.Run(); err == nil || err.Error() != expectedErrMessage {
		t.Errorf("Expected Error: %q , actual: %v", expectedErrMessage, err)
	}

	// read and restore stdout
	err := w.Close()
	if err != nil {
		t.Fatal(err)
	}
	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = backupStdout

	output := string(out)
	fmt.Println(output)

	if strings.Contains(output, hiddenDescription) {
		t.Errorf("The command must be hidden in the console: %s", output)
	}
}

func TestSubCommandHidden(t *testing.T) {
	// INIT
	rootConfig := newConfiguration()
	rootDefaultPointers := newDefaultPointersConfiguration()
	args := []string{
		"--help",
	}

	// root command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,
		Config:                rootConfig,
		DefaultPointersConfig: rootDefaultPointers,
		Run: func() error {
			return nil
		},
	}

	// hidden command
	hiddenDescription := "The Hidden command"
	versionConfig := &VersionConfig{Version: "0.1"}
	secretCmd := &Command{
		Name:                  "secret",
		Description:           hiddenDescription,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		HideHelp:              true,
		Run: func() error {
			return nil
		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})
	// add command Version
	flaeg.AddCommand(secretCmd)

	// catch stdout
	backupStdout := os.Stdout
	defer func() {
		os.Stdout = backupStdout
	}()
	r, w, _ := os.Pipe()
	os.Stdout = w

	// run test
	if err := flaeg.Run(); err == nil || err != pflag.ErrHelp {
		t.Errorf("Expected Error :help requested got Error : %v", err)
	}

	// read and restore stdout
	err := w.Close()
	if err != nil {
		t.Fatal(err)
	}
	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = backupStdout

	output := string(out)
	fmt.Println(output)

	if strings.Contains(output, hiddenDescription) {
		t.Errorf("The command must be hidden in the console: %s", output)
	}
}

// Test Commands feature with root and version commands
func TestCommandVersionUnknownCommand(t *testing.T) {
	// INIT
	rootConfig := newConfiguration()
	rootDefaultPointers := newDefaultPointersConfiguration()
	versionConfig := &VersionConfig{"0.1"}
	args := []string{
		"unknowncommand", // call Command
		"-h",
	}

	// root command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,

		Config:                rootConfig,
		DefaultPointersConfig: rootDefaultPointers,
		Run: func() error {
			fmt.Printf("Run with config :\n%+v\n", rootConfig)

			// CHECK
			check := newConfiguration()
			check.LogLevel = "INFO"
			check.Db = newDefaultPointersConfiguration().Db
			check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
			check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

			if !reflect.DeepEqual(rootConfig, check) {
				msg := fmt.Sprintf("\nexpected \t%+v \ngot \t\t%+v\n", check, rootConfig)
				return errors.New(msg)
			}
			return nil
		},
	}

	// version command
	versionCmd := &Command{
		Name:                  "version",
		Description:           `Print version`,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		Run: func() error {
			fmt.Printf("Version %s \n", versionConfig.Version)
			return nil

		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})
	// add command Version
	flaeg.AddCommand(versionCmd)

	// run test
	if err := flaeg.Run(); err == nil || (!strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "Command")) {
		t.Errorf("Expected Error :Command not found got Error : %s", err)
	}
}

func TestParseCommandVersionInitConfigNoDefaultAllFlag(t *testing.T) {
	// INIT
	rootConfig := newConfiguration()
	rootDefaultPointers := newDefaultPointersConfiguration()
	versionConfig := &VersionConfig{"0.1"}
	args := []string{
		"version", // call Command
		"-v2.2beta",
	}

	// root command
	rootCmd := &Command{
		Name: "flaegtest",
		Description: `flaegtest is a test program made to test flaeg library.
Complete documentation is available at https://github.com/containous/flaeg`,
		Config:                rootConfig,
		DefaultPointersConfig: rootDefaultPointers,
		Run: func() error {
			fmt.Printf("Run with config :\n%+v\n", rootConfig)

			// CHECK
			check := newConfiguration()
			check.LogLevel = "INFO"
			check.Db = newDefaultPointersConfiguration().Db
			check.Owner.Name = newDefaultPointersConfiguration().Owner.Name
			check.Owner.DateOfBirth, _ = time.Parse(time.RFC3339, "2016-04-20T17:39:00Z")

			if !reflect.DeepEqual(rootConfig, check) {
				msg := fmt.Sprintf("\nexpected \t%+v \ngot \t\t%+v\n", check, rootConfig)
				return errors.New(msg)
			}
			return nil
		},
	}

	// version command
	VersionCmd := &Command{
		Name:                  "version",
		Description:           `Print version`,
		Config:                versionConfig,
		DefaultPointersConfig: versionConfig,
		Run: func() error {
			fmt.Printf("Version %s \n", versionConfig.Version)

			// CHECK
			if versionConfig.Version != "2.2beta" {
				return fmt.Errorf("expected 2.2beta got %s", versionConfig.Version)
			}
			return nil

		},
	}

	// TEST
	// init flaeg
	flaeg := New(rootCmd, args)
	// add custom parser to flaeg
	flaeg.AddParser(reflect.TypeOf([]ServerInfo{}), &sliceServerValue{})
	// add command Version
	flaeg.AddCommand(VersionCmd)

	// run test
	cmd, err := flaeg.GetCommand()
	if err != nil {
		t.Fatal(err)
	}

	result, err := flaeg.Parse(cmd)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := &VersionConfig{"2.2beta"}

	if !reflect.DeepEqual(result.Config, check) {
		t.Errorf("\nexpected \t%+v \ngot \t\t%+v\n", check, result.Config)
	}
}

func TestSetPointersNilEmptyConfig(t *testing.T) {
	// run test
	config := &Configuration{}
	objVal := reflect.ValueOf(config)

	nilPointersConfig, err := setPointersNil(objVal)
	if err != nil {
		t.Fatal(err)
	}

	check := &Configuration{}
	if !reflect.DeepEqual(nilPointersConfig.Interface(), check) {
		t.Errorf("\nexpected \t%+v \ngot \t\t%+v\n", check, nilPointersConfig.Interface())
	}
	if !reflect.DeepEqual(objVal.Interface(), check) {
		t.Errorf("\nexpected \t%+v \ngot \t\t%+v\n", check, objVal.Interface())
	}
}

func TestSetPointersNilDefaultPointersConfig(t *testing.T) {
	// run test
	objVal := reflect.ValueOf(newDefaultPointersConfiguration())

	nilPointersConfig, err := setPointersNil(objVal)
	if err != nil {
		t.Fatal(err)
	}

	check := &Configuration{}
	if !reflect.DeepEqual(nilPointersConfig.Interface(), check) {
		t.Errorf("\nexpected \t%+v \ngot \t\t%+v\n", check, nilPointersConfig.Interface())
	}
	if !reflect.DeepEqual(objVal.Interface(), newDefaultPointersConfiguration()) {
		t.Errorf("\nexpected \t%+v \ngot \t\t%+v\n", newDefaultPointersConfiguration(), objVal.Interface())
	}
}

func TestSetPointersNilFullConfig(t *testing.T) {
	// init
	dob, _ := time.Parse(time.RFC3339, "1968-05-01T07:32:00Z")
	config := &Configuration{
		Name:     "Toto",
		LogLevel: "Tata",
		Timeout:  parse.Duration(time.Nanosecond),
		Db: &DatabaseInfo{
			ServerInfo: ServerInfo{
				Watch:  true,
				IP:     "192.168.87.78",
				Load:   5,
				Load64: 564,
			},
			ConnectionMax:   6,
			ConnectionMax64: 664,
		},
		Owner: &OwnerInfo{
			DateOfBirth: dob,
		},
	}
	objVal := reflect.ValueOf(config)

	// run test
	nilPointersConfig, err := setPointersNil(objVal)
	if err != nil {
		t.Fatal(err)
	}

	// check
	check := &Configuration{
		Name:     "Toto",
		LogLevel: "Tata",
		Timeout:  parse.Duration(time.Nanosecond),
	}
	if !reflect.DeepEqual(nilPointersConfig.Interface(), check) {
		t.Errorf("\nexpected \t%+v \ngot \t\t%+v\n", check, nilPointersConfig.Interface())
	}

	checkInit := &Configuration{
		Name:     "Toto",
		LogLevel: "Tata",
		Timeout:  parse.Duration(time.Nanosecond),
		Db: &DatabaseInfo{
			ServerInfo: ServerInfo{
				Watch:  true,
				IP:     "192.168.87.78",
				Load:   5,
				Load64: 564,
			},
			ConnectionMax:   6,
			ConnectionMax64: 664,
		},
		Owner: &OwnerInfo{
			DateOfBirth: dob,
		},
	}
	// cast
	initConfig, ok := objVal.Interface().(*Configuration)
	if !ok {
		t.Errorf("Cannot convert the config into Configuration")
	}

	if !reflect.DeepEqual(initConfig, checkInit) {
		fmt.Printf("expected \t%+v \ngot \t\t%+v\n", checkInit.Db, config.Db)
		fmt.Printf("expected \t%+v \ngot \t\t%+v\n", checkInit.Owner, config.Owner)
		t.Errorf("\nexpected \t%+v \ngot \t\t%+v\n", checkInit, objVal.Interface())
	}
}

type ConfigPointerField struct {
	PtrSubConfig *SubConfigWithUnexportedField `description:"pointer on a SubConfig with one unexported pointer field"`
}
type SubConfigWithUnexportedField struct {
	Exported        string `description:"Exported string field"`
	ptrSubSubConfig *SubSubConfig
}
type SubSubConfig struct {
	unexported string
}

func TestGetDefaultValueUnexportedFieldUnderPointer(t *testing.T) {
	// init
	config := &ConfigPointerField{}
	defaultPointersConfig := &ConfigPointerField{
		PtrSubConfig: &SubConfigWithUnexportedField{
			Exported: "ExportedSubFieldDefault",
		},
	}
	defaultValMap := make(map[string]reflect.Value)

	// TEST
	if err := getDefaultValue(reflect.ValueOf(config), reflect.ValueOf(defaultPointersConfig), defaultValMap, ""); err != nil {
		t.Fatal(err)
	}

	// check
	checkValue := map[string]reflect.Value{
		"ptrsubconfig":          reflect.ValueOf(&SubConfigWithUnexportedField{"ExportedSubFieldDefault", nil}),
		"ptrsubconfig.exported": reflect.ValueOf("ExportedSubFieldDefault"),
	}
	if len(checkValue) != len(defaultValMap) {
		t.Errorf("expected %d elements in defaultValMap got %d", len(checkValue), len(defaultValMap))
	}

	for flag, val := range defaultValMap {
		if !reflect.DeepEqual(checkValue[flag].Interface(), val.Interface()) {
			t.Errorf("flag %s : \nexpected \t%+v \ngot \t\t%+v\n", flag, checkValue[flag], val)
		}
	}
}

type ConfigWithUnexportedField struct {
	Exported string `description:"Exported string field"`
	other    string `description:"Non-exported string field"`
}

func TestGetTypesUnexported(t *testing.T) {
	config := &ConfigWithUnexportedField{}
	flagMap := make(map[string]reflect.StructField)

	err := getTypesRecursive(reflect.ValueOf(config), flagMap, "")

	checkErr := "field other is an unexported field"
	if err == nil || !strings.Contains(err.Error(), checkErr) {
		t.Errorf("expected error '%v' got '%v'", checkErr, err)
	}
}

func TestIsExported(t *testing.T) {
	checkTab := map[string]bool{
		"lowerCase": false,
		"UpperCase": true,
		"a":         false,
		"Z":         true,
	}
	for name, check := range checkTab {
		if exp := isExported(name); exp != check {
			t.Errorf("Expected %t got %t", check, exp)
		}
	}
}

func TestArgToLower(t *testing.T) {
	checkTab := map[string]string{
		"--lowercase":       "--lowercase",
		"-U":                "-u",
		"--CamelCase=TaTa":  "--camelcase=TaTa",
		"-UTaTa":            "-uTaTa",
		" --UPPERCASE":      "--uppercase",
		"  -U":              "-u",
		" --lowerCase=TaTa": "--lowercase=TaTa",
		"    -UTaTa":        "-uTaTa",
		"notAFlag":          "notAFlag",
		"-":                 "-",
		"--":                "--",
		"--A":               "--a",
	}
	for inArg, check := range checkTab {
		if outArg := argToLower(inArg); outArg != check {
			t.Errorf("inArg %s, Expected outArg %s got %s", inArg, check, outArg)
		}
	}
}

func TestArgsToLower(t *testing.T) {
	inArgs := []string{
		"--lowerCase",
		"-U",
		"--lowerCase=TaTa",
		"-uTaTa",
		" --lowerCase",
		"  -U",
		" --lowerCase=TaTa",
		"    -UTaTa",
		"notAFlag",
	}
	check := []string{
		"--lowercase",
		"-u",
		"--lowercase=TaTa",
		"-uTaTa",
		"--lowercase",
		"-u",
		"--lowercase=TaTa",
		"-uTaTa",
		"notAFlag",
	}
	if outArgs := argsToLower(inArgs); !reflect.DeepEqual(outArgs, check) {
		t.Errorf("Expected outArgs %s got %s", check, outArgs)
	}

}

func TestSplitArgs(t *testing.T) {
	inSlice := [][]string{
		{""},
		{"-a"},
		{"--arg=toto", "-atata"},
		{"cmd"},
		{"cmd", "-a"},
		{"cmd", "--arg=toto", "-atata"},
	}
	checkSlice := [][]string{
		{"", ""},
		{"", "-a"},
		{"", "--arg=toto", "-atata"},
		{"cmd"},
		{"cmd", "-a"},
		{"cmd", "--arg=toto", "-atata"},
	}
	for i, in := range inSlice {
		cmd, args := splitArgs(in)
		if cmd != checkSlice[i][0] {
			t.Errorf("Args %s, Expected cmd %s got %s", in, checkSlice[i][0], cmd)
		}
		if !reflect.DeepEqual(args, checkSlice[i][1:]) {
			t.Errorf("Args %s, Expected cmdArg %s got %s", in, checkSlice[i][1:], args)
		}
	}
}

func TestTypoPrintHelp(t *testing.T) {
	// init
	config := &struct {
		ShortDescription                                                     string `description:"shortDescription"`
		LoooooooooooooooooooooooooooooooooooooongFieldNameAndLongDescription string `description:"LoooooooooooooooooooooooooooooooooooooongFieldNameAndLongDescription has a very looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong description"`
		Sh                                                                   string `description:"short"`
	}{}

	flagMap := make(map[string]reflect.StructField)

	err := getTypesRecursive(reflect.ValueOf(config), flagMap, "")
	if err != nil {
		t.Fatal(err)
	}

	var stringParser parse.StringValue
	parsers := map[reflect.Type]parse.Parser{
		reflect.TypeOf(""): &stringParser,
	}
	defaultValMap := map[string]reflect.Value{}

	err = getDefaultValue(reflect.ValueOf(config), reflect.ValueOf(config), defaultValMap, "")
	if err != nil {
		t.Fatal(err)
	}

	// catch stdout
	backupStdout := os.Stdout
	defer func() {
		os.Stdout = backupStdout
	}()
	r, w, _ := os.Pipe()
	os.Stdout = w

	// test
	err = PrintHelp(flagMap, defaultValMap, parsers)
	if err != nil {
		t.Fatal(err)
	}

	// read and restore stdout
	err = w.Close()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = backupStdout

	// check
	const listFlagCheck = `LoooooooooooooooooooooooooooooooooooooongFieldNameAndLongDescription has a very looooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooong description`

	if strings.Contains(string(out), listFlagCheck) {
		t.Errorf("Expected help description splitted on many line")
	}
}
