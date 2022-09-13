package reporters_test

import (
	"reflect"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/internal"
	"github.com/onsi/ginkgo/v2/internal/test_helpers"
	"github.com/onsi/ginkgo/v2/reporters"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/gbytes"
)

type StackTrace string

const DELIMITER = `{{gray}}------------------------------{{/}}`

var cl0 = types.CodeLocation{FileName: "cl0.go", LineNumber: 12, FullStackTrace: "full-trace\ncl-0"}
var cl1 = types.CodeLocation{FileName: "cl1.go", LineNumber: 37, FullStackTrace: "full-trace\ncl-1"}
var cl2 = types.CodeLocation{FileName: "cl2.go", LineNumber: 80, FullStackTrace: "full-trace\ncl-2"}
var cl3 = types.CodeLocation{FileName: "cl3.go", LineNumber: 103, FullStackTrace: "full-trace\ncl-3"}
var cl4 = types.CodeLocation{FileName: "cl4.go", LineNumber: 144, FullStackTrace: "full-trace\ncl-4"}

func CLS(cls ...types.CodeLocation) []types.CodeLocation { return cls }
func CTS(componentTexts ...string) []string              { return componentTexts }
func CLabels(labels ...Labels) []Labels                  { return labels }

type FailureNodeLocation types.CodeLocation
type ForwardedPanic string

var PLACEHOLDER_TIME = time.Now()
var FORMATTED_TIME = PLACEHOLDER_TIME.Format(types.GINKGO_TIME_FORMAT)

// convenience helper to quickly make Failures
func F(options ...interface{}) types.Failure {
	failure := types.Failure{}
	for _, option := range options {
		switch reflect.TypeOf(option) {
		case reflect.TypeOf(""):
			failure.Message = option.(string)
		case reflect.TypeOf(types.CodeLocation{}):
			failure.Location = option.(types.CodeLocation)
		case reflect.TypeOf(ForwardedPanic("")):
			failure.ForwardedPanic = string(option.(ForwardedPanic))
		case reflect.TypeOf(types.FailureNodeInContainer):
			failure.FailureNodeContext = option.(types.FailureNodeContext)
		case reflect.TypeOf(0):
			failure.FailureNodeContainerIndex = option.(int)
		case reflect.TypeOf(FailureNodeLocation{}):
			failure.FailureNodeLocation = types.CodeLocation(option.(FailureNodeLocation))
		case reflect.TypeOf(types.NodeTypeIt):
			failure.FailureNodeType = option.(types.NodeType)
		case reflect.TypeOf(types.ProgressReport{}):
			failure.ProgressReport = option.(types.ProgressReport)
		}
	}
	return failure
}

type STD string
type GW string

// convenience helper to quickly make summaries
func S(options ...interface{}) types.SpecReport {
	report := types.SpecReport{
		LeafNodeType: types.NodeTypeIt,
		State:        types.SpecStatePassed,
		NumAttempts:  1,
		RunTime:      time.Second,
	}
	for _, option := range options {
		switch reflect.TypeOf(option) {
		case reflect.TypeOf([]string{}):
			report.ContainerHierarchyTexts = option.([]string)
		case reflect.TypeOf([]types.CodeLocation{}):
			report.ContainerHierarchyLocations = option.([]types.CodeLocation)
		case reflect.TypeOf([]Labels{}):
			report.ContainerHierarchyLabels = [][]string{}
			for _, labels := range option.([]Labels) {
				report.ContainerHierarchyLabels = append(report.ContainerHierarchyLabels, []string(labels))
			}
		case reflect.TypeOf(""):
			report.LeafNodeText = option.(string)
		case reflect.TypeOf(types.NodeTypeIt):
			report.LeafNodeType = option.(types.NodeType)
		case reflect.TypeOf(types.CodeLocation{}):
			report.LeafNodeLocation = option.(types.CodeLocation)
		case reflect.TypeOf(Labels{}):
			report.LeafNodeLabels = []string(option.(Labels))
		case reflect.TypeOf(types.SpecStatePassed):
			report.State = option.(types.SpecState)
		case reflect.TypeOf(time.Second):
			report.RunTime = option.(time.Duration)
		case reflect.TypeOf(types.Failure{}):
			report.Failure = option.(types.Failure)
		case reflect.TypeOf(0):
			report.NumAttempts = option.(int)
		case reflect.TypeOf(STD("")):
			report.CapturedStdOutErr = string(option.(STD))
		case reflect.TypeOf(GW("")):
			report.CapturedGinkgoWriterOutput = string(option.(GW))
		case reflect.TypeOf(types.ReportEntry{}):
			report.ReportEntries = append(report.ReportEntries, option.(types.ReportEntry))
		}
	}
	if len(report.ContainerHierarchyLabels) == 0 {
		for range report.ContainerHierarchyTexts {
			report.ContainerHierarchyLabels = append(report.ContainerHierarchyLabels, []string{})
		}
	}
	return report
}

func RE(name string, cl types.CodeLocation, args ...interface{}) types.ReportEntry {
	entry, _ := internal.NewReportEntry(name, cl, args...)
	entry.Time = PLACEHOLDER_TIME
	return entry
}

type ConfigFlags uint8

const (
	Succinct ConfigFlags = 1 << iota
	Verbose
	VeryVerbose
	ReportPassed
	FullTrace
)

func (cf ConfigFlags) Has(flag ConfigFlags) bool { return cf&flag != 0 }

func C(flags ...ConfigFlags) types.ReporterConfig {
	f := ConfigFlags(0)
	if len(flags) > 0 {
		f = flags[0]
	}
	Ω(f.Has(Verbose) && f.Has(Succinct)).Should(BeFalse(), "Setting more than one of Succinct, Verbose, or VeryVerbose is a configuration error")
	Ω(f.Has(VeryVerbose) && f.Has(Succinct)).Should(BeFalse(), "Setting more than one of Succinct, Verbose, or VeryVerbose is a configuration error")
	Ω(f.Has(VeryVerbose) && f.Has(Verbose)).Should(BeFalse(), "Setting more than one of Succinct, Verbose, or VeryVerbose is a configuration error")
	return types.ReporterConfig{
		NoColor:                true,
		SlowSpecThreshold:      SlowSpecThreshold,
		Succinct:               f.Has(Succinct),
		Verbose:                f.Has(Verbose),
		VeryVerbose:            f.Has(VeryVerbose),
		AlwaysEmitGinkgoWriter: f.Has(ReportPassed),
		FullTrace:              f.Has(FullTrace),
	}
}

type CurrentNodeText string
type CurrentStepText string

func PR(options ...interface{}) types.ProgressReport {
	report := types.ProgressReport{
		ParallelProcess:   1,
		RunningInParallel: false,

		SpecStartTime:        time.Now().Add(-5 * time.Second),
		CurrentNodeStartTime: time.Now().Add(-3 * time.Second),
		CurrentStepStartTime: time.Now().Add(-1 * time.Second),

		LeafNodeLocation:    cl0,
		CurrentNodeLocation: cl1,
		CurrentStepLocation: cl2,
	}
	for _, option := range options {
		switch reflect.TypeOf(option) {
		case reflect.TypeOf([]string{}):
			report.ContainerHierarchyTexts = option.([]string)
		case reflect.TypeOf(""):
			report.LeafNodeText = option.(string)
		case reflect.TypeOf(types.NodeTypeInvalid):
			report.CurrentNodeType = option.(types.NodeType)
		case reflect.TypeOf(CurrentNodeText("")):
			report.CurrentNodeText = string(option.(CurrentNodeText))
		case reflect.TypeOf(CurrentStepText("")):
			report.CurrentStepText = string(option.(CurrentStepText))
		case reflect.TypeOf(types.Goroutine{}):
			report.Goroutines = append(report.Goroutines, option.(types.Goroutine))
		case reflect.TypeOf([]types.Goroutine{}):
			report.Goroutines = append(report.Goroutines, option.([]types.Goroutine)...)
		case reflect.TypeOf(0):
			report.ParallelProcess = option.(int)
		case reflect.TypeOf(true):
			report.RunningInParallel = option.(bool)
		}
	}
	return report
}

func Fn(f string, filename string, line int64, highlight ...bool) types.FunctionCall {
	isHighlight := false
	if len(highlight) > 0 && highlight[0] {
		isHighlight = true
	}
	return types.FunctionCall{
		Function:  f,
		Filename:  filename,
		Line:      line,
		Highlight: isHighlight,
	}
}

func G(options ...interface{}) types.Goroutine {
	goroutine := types.Goroutine{
		ID:              17,
		State:           "running",
		IsSpecGoroutine: false,
	}

	for _, option := range options {
		switch reflect.TypeOf(option) {
		case reflect.TypeOf(true):
			goroutine.IsSpecGoroutine = option.(bool)
		case reflect.TypeOf(""):
			goroutine.State = option.(string)
		case reflect.TypeOf(types.FunctionCall{}):
			goroutine.Stack = append(goroutine.Stack, option.(types.FunctionCall))
		case reflect.TypeOf([]types.FunctionCall{}):
			goroutine.Stack = append(goroutine.Stack, option.([]types.FunctionCall)...)
		}
	}

	return goroutine
}

const SlowSpecThreshold = 3 * time.Second

type REGEX string

var _ = Describe("DefaultReporter", func() {
	var DENOTER = "•"
	var RETRY_DENOTER = "↺"
	if runtime.GOOS == "windows" {
		DENOTER = "+"
		RETRY_DENOTER = "R"
	}

	var buf *gbytes.Buffer
	verifyExpectedOutput := func(expected []string) {
		if len(expected) == 0 {
			ExpectWithOffset(1, buf.Contents()).Should(BeEmpty())
		} else {
			ExpectWithOffset(1, string(buf.Contents())).Should(Equal(strings.Join(expected, "\n")), test_helpers.MultilineTextHelper(string(buf.Contents())))
		}
	}

	verifyRegExOutput := func(expected []interface{}) {
		if len(expected) == 0 {
			ExpectWithOffset(1, buf.Contents()).Should(BeEmpty())
			return
		}

		summary := test_helpers.MultilineTextHelper(string(buf.Contents()))
		lines := strings.Split(string(buf.Contents()), "\n")
		Expect(len(expected)).Should(BeNumerically("<=", len(lines)), summary)
		for idx, expected := range expected {
			switch v := expected.(type) {
			case REGEX:
				ExpectWithOffset(1, lines[idx]).Should(MatchRegexp(string(v)), summary)
			default:
				ExpectWithOffset(1, lines[idx]).Should(Equal(v), summary)
			}
		}
	}

	BeforeEach(func() {
		buf = gbytes.NewBuffer()
		format.CharactersAroundMismatchToInclude = 100
	})

	DescribeTable("Rendering SuiteWillBegin",
		func(conf types.ReporterConfig, report types.Report, expected ...string) {
			reporter := reporters.NewDefaultReporterUnderTest(conf, buf)
			reporter.SuiteWillBegin(report)
			verifyExpectedOutput(expected)
		},
		Entry("Default Behavior",
			C(),
			types.Report{
				SuiteDescription: "My Suite", SuitePath: "/path/to/suite", PreRunStats: types.PreRunStats{SpecsThatWillRun: 15, TotalSpecs: 20},
				SuiteConfig: types.SuiteConfig{RandomSeed: 17, ParallelTotal: 1},
			},
			"Running Suite: My Suite - /path/to/suite",
			"========================================",
			"Random Seed: {{bold}}17{{/}}",
			"",
			"Will run {{bold}}15{{/}} of {{bold}}20{{/}} specs",
			"",
		),
		Entry("With Labels",
			C(),
			types.Report{
				SuiteDescription: "My Suite", SuitePath: "/path/to/suite", SuiteLabels: []string{"dog", "fish"}, PreRunStats: types.PreRunStats{SpecsThatWillRun: 15, TotalSpecs: 20},
				SuiteConfig: types.SuiteConfig{RandomSeed: 17, ParallelTotal: 1},
			},
			"Running Suite: My Suite - /path/to/suite",
			"{{coral}}[dog, fish]{{/}} ",
			"========================================",
			"Random Seed: {{bold}}17{{/}}",
			"",
			"Will run {{bold}}15{{/}} of {{bold}}20{{/}} specs",
			"",
		),
		Entry("With long Labels",
			C(),
			types.Report{
				SuiteDescription: "My Suite", SuitePath: "/path/to/suite", SuiteLabels: []string{"dog", "fish", "kalamazoo", "kangaroo", "chicken", "asparagus"}, PreRunStats: types.PreRunStats{SpecsThatWillRun: 15, TotalSpecs: 20},
				SuiteConfig: types.SuiteConfig{RandomSeed: 17, ParallelTotal: 1},
			},
			"Running Suite: My Suite - /path/to/suite",
			"{{coral}}[dog, fish, kalamazoo, kangaroo, chicken, asparagus]{{/}} ",
			"====================================================",
			"Random Seed: {{bold}}17{{/}}",
			"",
			"Will run {{bold}}15{{/}} of {{bold}}20{{/}} specs",
			"",
		),
		Entry("When configured to randomize all specs",
			C(),
			types.Report{
				SuiteDescription: "My Suite", SuitePath: "/path/to/suite", PreRunStats: types.PreRunStats{SpecsThatWillRun: 15, TotalSpecs: 20},
				SuiteConfig: types.SuiteConfig{RandomSeed: 17, ParallelTotal: 1, RandomizeAllSpecs: true},
			},
			"Running Suite: My Suite - /path/to/suite",
			"========================================",
			"Random Seed: {{bold}}17{{/}} - will randomize all specs",
			"",
			"Will run {{bold}}15{{/}} of {{bold}}20{{/}} specs",
			"",
		),
		Entry("when configured to run in parallel",
			C(),
			types.Report{
				SuiteDescription: "My Suite", SuitePath: "/path/to/suite", PreRunStats: types.PreRunStats{SpecsThatWillRun: 15, TotalSpecs: 20},
				SuiteConfig: types.SuiteConfig{RandomSeed: 17, ParallelTotal: 3},
			},
			"Running Suite: My Suite - /path/to/suite",
			"========================================",
			"Random Seed: {{bold}}17{{/}}",
			"",
			"Will run {{bold}}15{{/}} of {{bold}}20{{/}} specs",
			"Running in parallel across {{bold}}3{{/}} processes",
			"",
		),
		Entry("when succinct and in series",
			C(Succinct),
			types.Report{
				SuiteDescription: "My Suite", SuitePath: "/path/to/suite", PreRunStats: types.PreRunStats{SpecsThatWillRun: 15, TotalSpecs: 20},
				SuiteConfig: types.SuiteConfig{RandomSeed: 17, ParallelTotal: 1},
			},
			"[17] {{bold}}My Suite{{/}} - 15/20 specs ",
		),
		Entry("when succinct and in parallel",
			C(Succinct),
			types.Report{
				SuiteDescription: "My Suite", SuitePath: "/path/to/suite", PreRunStats: types.PreRunStats{SpecsThatWillRun: 15, TotalSpecs: 20},
				SuiteConfig: types.SuiteConfig{RandomSeed: 17, ParallelTotal: 3},
			},
			"[17] {{bold}}My Suite{{/}} - 15/20 specs - 3 procs ",
		),

		Entry("when succinct and with labels",
			C(Succinct),
			types.Report{
				SuiteDescription: "My Suite", SuitePath: "/path/to/suite", SuiteLabels: Label("dog, fish"), PreRunStats: types.PreRunStats{SpecsThatWillRun: 15, TotalSpecs: 20},
				SuiteConfig: types.SuiteConfig{RandomSeed: 17, ParallelTotal: 3},
			},
			"[17] {{bold}}My Suite{{/}} {{coral}}[dog, fish]{{/}} - 15/20 specs - 3 procs ",
		),
	)

	DescribeTable("WillRun",
		func(conf types.ReporterConfig, report types.SpecReport, output ...string) {
			reporter := reporters.NewDefaultReporterUnderTest(conf, buf)
			reporter.WillRun(report)
			verifyExpectedOutput(output)
		},
		Entry("when not verbose, it emits nothing", C(), S(CTS("A"), CLS(cl0))),
		Entry("pending specs are not emitted", C(Verbose), S(types.SpecStatePending)),
		Entry("skipped specs are not emitted", C(Verbose), S(types.SpecStateSkipped)),
		Entry("setup nodes", C(Verbose),
			S(types.NodeTypeBeforeSuite, cl0),
			DELIMITER,
			"{{bold}}[BeforeSuite] {{/}}",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
		),
		Entry("ReportAfterSuite nodes", C(Verbose),
			S("my report", cl0, types.NodeTypeReportAfterSuite),
			DELIMITER,
			"{{bold}}[ReportAfterSuite] my report{{/}}",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
		),
		Entry("top-level it nodes", C(Verbose),
			S("My Test", cl0),
			DELIMITER,
			"{{bold}}My Test{{/}}",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
		),
		Entry("nested it nodes", C(Verbose),
			S(CTS("Container", "Nested Container"), "My Test", CLS(cl0, cl1), cl2),
			DELIMITER,
			"{{/}}Container {{gray}}Nested Container{{/}}",
			"  {{bold}}My Test{{/}}",
			"  {{gray}}"+cl2.String()+"{{/}}",
			"",
		),
		Entry("specs with labels", C(Verbose),
			S(CTS("Container", "Nested Container"), "My Test", CLS(cl0, cl1), cl2, CLabels(Label("dog", "cat"), Label("cat", "fruit")), Label("giraffe", "gorilla", "cat")),
			DELIMITER,
			"{{/}}Container {{gray}}Nested Container{{/}}",
			"  {{bold}}My Test{{/}} {{coral}}[dog, cat, fruit, giraffe, gorilla]{{/}}",
			"  {{gray}}"+cl2.String()+"{{/}}",
			"",
		),
	)

	DescribeTable("DidRun",
		func(conf types.ReporterConfig, report types.SpecReport, output ...interface{}) {
			reporter := reporters.NewDefaultReporterUnderTest(conf, buf)
			reporter.DidRun(report)
			verifyRegExOutput(output)
		},
		// Passing Tests
		Entry("a passing test",
			C(),
			S("A", cl0),
			"{{green}}"+DENOTER+"{{/}}",
		),
		Entry("a passing test that was retried",
			C(),
			S(CTS("A"), "B", CLS(cl0), cl1, 2),
			DELIMITER,
			"{{green}}"+RETRY_DENOTER+" [FLAKEY TEST - TOOK 2 ATTEMPTS TO PASS] [1.000 seconds]{{/}}",
			"{{/}}A {{gray}}B{{/}}",
			"{{gray}}"+cl1.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing test that has ginkgo writer output and/or non-visible report entries",
			C(),
			S("A", cl0, GW("GINKGO-WRITER-OUTPUT"), RE("fail-report-name", cl1, types.ReportEntryVisibilityFailureOrVerbose), RE("hidden-report-name", cl2, types.ReportEntryVisibilityNever)),
			"{{green}}"+DENOTER+"{{/}}",
		),
		Entry("a passing test that has ginkgo writer output, with ReportPassed configured",
			C(ReportPassed),
			S(CTS("A"), "B", CLS(cl0), cl1, GW("GINKGO-WRITER-OUTPUT\nSHOULD EMIT")),
			DELIMITER,
			"{{green}}"+DENOTER+" [1.000 seconds]{{/}}",
			"{{/}}A {{gray}}B{{/}}",
			"{{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GINKGO-WRITER-OUTPUT",
			"    SHOULD EMIT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing test that has ginkgo writer output and a FailurOrVerbose entry, with Verbose configured",
			C(Verbose),
			S("A", cl0, GW("GINKGO-WRITER-OUTPUT\nSHOULD EMIT"), RE("failure-or-verbose-report-name", cl1, types.ReportEntryVisibilityFailureOrVerbose), RE("hidden-report-name", cl2, types.ReportEntryVisibilityNever)),
			DELIMITER,
			"{{green}}"+DENOTER+" [1.000 seconds]{{/}}",
			"A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GINKGO-WRITER-OUTPUT",
			"    SHOULD EMIT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{gray}}Begin Report Entries >>{{/}}",
			"    {{bold}}failure-or-verbose-report-name{{gray}} - "+cl1.String()+" @ "+FORMATTED_TIME+"{{/}}",
			"  {{gray}}<< End Report Entries{{/}}",
			DELIMITER,
			"",
		),
		Entry("a slow passing test",
			C(),
			S(CTS("A"), "B", CLS(cl0), cl1, time.Minute, GW("GINKGO-WRITER-OUTPUT")),
			DELIMITER,
			"{{green}}"+DENOTER+" [SLOW TEST] [60.000 seconds]{{/}}",
			"{{/}}A {{gray}}B{{/}}",
			"{{gray}}"+cl1.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing test with captured stdout",
			C(),
			S(CTS("A"), "B", CLS(cl0), cl1, GW("GINKGO-WRITER-OUTPUT"), STD("STD-OUTPUT\nSHOULD EMIT")),
			DELIMITER,
			"{{green}}"+DENOTER+" [1.000 seconds]{{/}}",
			"{{/}}A {{gray}}B{{/}}",
			"{{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    SHOULD EMIT",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing test with a ReportEntry that is always visible",
			C(),
			S(CTS("A"), "B", CLS(cl0), cl1, GW("GINKGO-WRITER-OUTPUT"), RE("report-name", cl2, "report-content"), RE("other-report-name", cl3), RE("fail-report-name", cl4, types.ReportEntryVisibilityFailureOrVerbose)),
			DELIMITER,
			"{{green}}"+DENOTER+" [1.000 seconds]{{/}}",
			"{{/}}A {{gray}}B{{/}}",
			"{{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}Begin Report Entries >>{{/}}",
			"    {{bold}}report-name{{gray}} - "+cl2.String()+" @ "+FORMATTED_TIME+"{{/}}",
			"      report-content",
			"    {{bold}}other-report-name{{gray}} - "+cl3.String()+" @ "+FORMATTED_TIME+"{{/}}",
			"  {{gray}}<< End Report Entries{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing suite setup emits nothing",
			C(),
			S(types.NodeTypeBeforeSuite, cl0, GW("GINKGO-WRITER-OUTPUT")),
		),
		Entry("a passing suite setup with verbose always emits",
			C(Verbose),
			S(types.NodeTypeBeforeSuite, cl0, GW("GINKGO-WRITER-OUTPUT")),
			DELIMITER,
			"{{green}}[BeforeSuite] PASSED [1.000 seconds]{{/}}",
			"[BeforeSuite] ",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GINKGO-WRITER-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing suite setup with captured stdout always emits",
			C(),
			S(types.NodeTypeBeforeSuite, cl0, STD("STD-OUTPUT")),
			DELIMITER,
			"{{green}}[BeforeSuite] PASSED [1.000 seconds]{{/}}",
			"{{/}}[BeforeSuite] {{/}}",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing ReportAfterSuite emits nothing",
			C(),
			S("my report", types.NodeTypeReportAfterSuite, cl0, GW("GINKGO-WRITER-OUTPUT")),
		),
		Entry("a passing ReportAfterSuite with verbose always emits",
			C(Verbose),
			S("my report", types.NodeTypeReportAfterSuite, cl0, GW("GINKGO-WRITER-OUTPUT")),
			DELIMITER,
			"{{green}}[ReportAfterSuite] PASSED [1.000 seconds]{{/}}",
			"[ReportAfterSuite] my report",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GINKGO-WRITER-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing ReportAfterSuite with captured stdout always emits",
			C(),
			S("my report", types.NodeTypeReportAfterSuite, cl0, STD("STD-OUTPUT")),
			DELIMITER,
			"{{green}}[ReportAfterSuite] PASSED [1.000 seconds]{{/}}",
			"{{/}}[ReportAfterSuite] my report{{/}}",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing ReportAfterSuite with verbose always emits",
			C(Verbose),
			S("my report", types.NodeTypeReportAfterSuite, cl0, GW("GINKGO-WRITER-OUTPUT")),
			DELIMITER,
			"{{green}}[ReportAfterSuite] PASSED [1.000 seconds]{{/}}",
			"[ReportAfterSuite] my report",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GINKGO-WRITER-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			DELIMITER,
			"",
		),
		Entry("a passing Suite-level DeferCleanup emits nothing",
			C(),
			S(types.NodeTypeCleanupAfterSuite, cl0, GW("GINKGO-WRITER-OUTPUT")),
		),
		Entry("a passing Suite-level DeferCleanup with verbose always emits",
			C(Verbose),
			S(types.NodeTypeCleanupAfterSuite, cl0, GW("GINKGO-WRITER-OUTPUT")),
			DELIMITER,
			"{{green}}[DeferCleanup (Suite)] PASSED [1.000 seconds]{{/}}",
			"[DeferCleanup (Suite)] ",
			"{{gray}}cl0.go:12{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GINKGO-WRITER-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			DELIMITER,
			"",
		),

		// Pending Tests
		Entry("a pending test when succinct",
			C(Succinct),
			S("A", cl0, types.SpecStatePending, GW("GW-OUTPUT"), STD("STD-OUTPUT")),
			"{{yellow}}P{{/}}",
		),
		Entry("a pending test normally",
			C(),
			S("A", cl0, types.SpecStatePending, GW("GW-OUTPUT")),
			DELIMITER,
			"{{yellow}}P [PENDING]{{/}}",
			"{{/}}A{{/}}",
			"{{gray}}cl0.go:12{{/}}",
			DELIMITER,
			"",
		),

		Entry("a pending test when verbose",
			C(Verbose),
			S("A", cl0, types.SpecStatePending, GW("GW-OUTPUT")),
			DELIMITER,
			"{{yellow}}P [PENDING]{{/}}",
			"{{/}}A{{/}}",
			"{{gray}}cl0.go:12{{/}}",
			DELIMITER,
			"",
		),
		Entry("a pending test when very verbose",
			C(VeryVerbose),
			S(CTS("A"), "B", CLS(cl0), cl1, types.SpecStatePending, GW("GW-OUTPUT"), STD("STD-OUTPUT")),
			DELIMITER,
			"{{yellow}}P [PENDING]{{/}}",
			"A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  B",
			"  {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			DELIMITER,
			"",
		),
		// Skipped Tests
		Entry("a skipped test without a failure message when succinct",
			C(Succinct),
			S("A", cl0, types.SpecStateSkipped, GW("GW-OUTPUT")),
			"{{cyan}}S{{/}}",
		),
		Entry("a skipped test without a failure message",
			C(),
			S("A", cl0, types.SpecStateSkipped, GW("GW-OUTPUT")),
			"{{cyan}}S{{/}}",
		),
		Entry("a skipped test without a failure message when verbose",
			C(Verbose),
			S("A", cl0, types.SpecStateSkipped, GW("GW-OUTPUT")),
			"{{cyan}}S{{/}}",
		),
		Entry("a skipped test without a failure message when very verbose",
			C(VeryVerbose),
			S("A", cl0, types.SpecStateSkipped, GW("GW-OUTPUT")),
			"{{gray}}------------------------------{{/}}",
			"{{cyan}}S [SKIPPED] [1.000 seconds]{{/}}",
			"A",
			"{{gray}}cl0.go:12{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"{{gray}}------------------------------{{/}}",
			"",
		),
		Entry("a skipped test with a failure message when succinct",
			C(Succinct),
			S(CTS("A"), "B", CLS(cl0), cl1, types.SpecStateSkipped, GW("GW-OUTPUT"), STD("STD-OUTPUT"),
				F("user skipped", types.FailureNodeIsLeafNode, types.NodeTypeIt, FailureNodeLocation(cl1), cl2),
			),
			DELIMITER,
			"{{cyan}}S [SKIPPED] [1.000 seconds]{{/}}",
			"{{/}}A {{gray}}{{cyan}}{{bold}}[It] B{{/}}{{/}}",
			"{{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{cyan}}user skipped{{/}}",
			"  {{cyan}}In {{bold}}[It]{{/}}{{cyan}} at: {{bold}}"+cl2.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("a skipped test with a failure message and normal verbosity",
			C(),
			S(CTS("A"), "B", CLS(cl0), cl1, types.SpecStateSkipped, GW("GW-OUTPUT"), STD("STD-OUTPUT"),
				F("user skipped", types.FailureNodeIsLeafNode, types.NodeTypeIt, FailureNodeLocation(cl1), cl2),
			),
			DELIMITER,
			"{{cyan}}S [SKIPPED] [1.000 seconds]{{/}}",
			"A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{cyan}}{{bold}}[It] B{{/}}",
			"  {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{cyan}}user skipped{{/}}",
			"  {{cyan}}In {{bold}}[It]{{/}}{{cyan}} at: {{bold}}"+cl2.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("a skipped test with a failure message and verbose",
			C(Verbose),
			S(CTS("A"), "B", CLS(cl0), cl1, types.SpecStateSkipped, GW("GW-OUTPUT"), STD("STD-OUTPUT"),
				F("user skipped", types.FailureNodeIsLeafNode, types.NodeTypeIt, FailureNodeLocation(cl1), cl2),
			),
			DELIMITER,
			"{{cyan}}S [SKIPPED] [1.000 seconds]{{/}}",
			"A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{cyan}}{{bold}}[It] B{{/}}",
			"  {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{cyan}}user skipped{{/}}",
			"  {{cyan}}In {{bold}}[It]{{/}}{{cyan}} at: {{bold}}"+cl2.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("a skipped test with a failure message and very verbose",
			C(VeryVerbose),
			S(CTS("A"), "B", CLS(cl0), cl1, types.SpecStateSkipped, GW("GW-OUTPUT"), STD("STD-OUTPUT"),
				F("user skipped", types.FailureNodeIsLeafNode, types.NodeTypeIt, FailureNodeLocation(cl1), cl2),
			),
			DELIMITER,
			"{{cyan}}S [SKIPPED] [1.000 seconds]{{/}}",
			"A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{cyan}}{{bold}}[It] B{{/}}",
			"  {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{cyan}}user skipped{{/}}",
			"  {{cyan}}In {{bold}}[It]{{/}}{{cyan}} at: {{bold}}"+cl2.String()+"{{/}}",
			DELIMITER,
			"",
		),
		//Failed tests
		Entry("when a test has failed in an It",
			C(),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2, CLabels(Label("dog", "cat"), Label("cat", "cow")),
				types.SpecStateFailed, 2,
				Label("cow", "fish"),
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, types.NodeTypeIt, FailureNodeLocation(cl2), cl3),
				RE("report-name", cl4, "report-content"),
				RE("fail-report-name", cl4, "fail-report-content", types.ReportEntryVisibilityFailureOrVerbose),
				RE("hidden-report-name", cl4, "hidden-report-content", types.ReportEntryVisibilityNever),
			),
			DELIMITER,
			"{{red}}"+DENOTER+" [FAILED] [1.000 seconds]{{/}}",
			"Describe A {{coral}}[dog, cat]{{/}}",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  Context B {{coral}}[cat, cow]{{/}}",
			"  {{gray}}"+cl1.String()+"{{/}}",
			"    {{red}}{{bold}}[It] The Test{{/}} {{coral}}[cow, fish]{{/}}",
			"    {{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{gray}}Begin Report Entries >>{{/}}",
			"    {{bold}}report-name{{gray}} - "+cl4.String()+" @ "+FORMATTED_TIME+"{{/}}",
			"      report-content",
			"    {{bold}}fail-report-name{{gray}} - "+cl4.String()+" @ "+FORMATTED_TIME+"{{/}}",
			"      fail-report-content",
			"  {{gray}}<< End Report Entries{{/}}",
			"",
			"  {{red}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{red}}In {{bold}}[It]{{/}}{{red}} at: {{bold}}"+cl3.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("when a test has failed in a setup/teardown node",
			C(),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
				types.SpecStateFailed, 2,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4),
			),
			DELIMITER,
			"{{red}}"+DENOTER+" [FAILED] [1.000 seconds]{{/}}",
			"Describe A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{red}}{{bold}}Context B [JustBeforeEach]{{/}}",
			"  {{gray}}"+cl3.String()+"{{/}}",
			"    The Test",
			"    {{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{red}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{red}}In {{bold}}[JustBeforeEach]{{/}}{{red}} at: {{bold}}"+cl4.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("when a test has failed in a setup/teardown node defined at the top level",
			C(),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
				types.SpecStateFailed, 2,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeAtTopLevel, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4),
			),
			DELIMITER,
			"{{red}}"+DENOTER+" [FAILED] [1.000 seconds]{{/}}",
			"{{red}}{{bold}}TOP-LEVEL [JustBeforeEach]{{/}}",
			"{{gray}}"+cl3.String()+"{{/}}",
			"  Describe A",
			"  {{gray}}"+cl0.String()+"{{/}}",
			"    Context B",
			"    {{gray}}"+cl1.String()+"{{/}}",
			"      The Test",
			"      {{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{red}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{red}}In {{bold}}[JustBeforeEach]{{/}}{{red}} at: {{bold}}"+cl4.String()+"{{/}}",
			DELIMITER,
			""),
		Entry("when a test has failed in a setup/teardown node and Succinct is configured",
			C(Succinct),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
				types.SpecStateFailed, 2,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4),
			),
			DELIMITER,
			"{{red}}"+DENOTER+" [FAILED] [1.000 seconds]{{/}}",
			"{{/}}Describe A {{gray}}{{red}}{{bold}}Context B [JustBeforeEach]{{/}} {{/}}The Test{{/}}",
			"{{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{red}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{red}}In {{bold}}[JustBeforeEach]{{/}}{{red}} at: {{bold}}"+cl4.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("when a test has failed and FullTrace is configured",
			C(FullTrace),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
				types.SpecStateFailed, 2,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4),
			),
			DELIMITER,
			"{{red}}"+DENOTER+" [FAILED] [1.000 seconds]{{/}}",
			"Describe A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{red}}{{bold}}Context B [JustBeforeEach]{{/}}",
			"  {{gray}}"+cl3.String()+"{{/}}",
			"    The Test",
			"    {{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{red}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{red}}In {{bold}}[JustBeforeEach]{{/}}{{red}} at: {{bold}}"+cl4.String()+"{{/}}",
			"",
			"  {{red}}Full Stack Trace{{/}}",
			"    full-trace",
			"    cl-4",
			DELIMITER,
			"",
		),
		Entry("when a suite setup node has failed",
			C(),
			S(types.NodeTypeSynchronizedBeforeSuite, cl0, types.SpecStateFailed, 1,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, FailureNodeLocation(cl0), types.NodeTypeSynchronizedBeforeSuite, 1, cl1),
			),
			DELIMITER,
			"{{red}}[SynchronizedBeforeSuite] [FAILED] [1.000 seconds]{{/}}",
			"{{red}}{{bold}}[SynchronizedBeforeSuite] {{/}}",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{red}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{red}}In {{bold}}[SynchronizedBeforeSuite]{{/}}{{red}} at: {{bold}}"+cl1.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("when a ReportAfterSuite node has failed",
			C(),
			S("my report", cl0, types.NodeTypeReportAfterSuite, types.SpecStateFailed, 1,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, FailureNodeLocation(cl0), types.NodeTypeReportAfterSuite, 1, cl1),
			),
			DELIMITER,
			"{{red}}[ReportAfterSuite] [FAILED] [1.000 seconds]{{/}}",
			"{{red}}{{bold}}[ReportAfterSuite] my report{{/}}",
			"{{gray}}"+cl0.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{red}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{red}}In {{bold}}[ReportAfterSuite]{{/}}{{red}} at: {{bold}}"+cl1.String()+"{{/}}",
			DELIMITER,
			"",
		),

		Entry("when a test has panicked and there is no forwarded panic",
			C(),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
				types.SpecStatePanicked, 2,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4),
			),
			DELIMITER,
			"{{magenta}}"+DENOTER+"! [PANICKED] [1.000 seconds]{{/}}",
			"Describe A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{magenta}}{{bold}}Context B [JustBeforeEach]{{/}}",
			"  {{gray}}"+cl3.String()+"{{/}}",
			"    The Test",
			"    {{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{magenta}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{magenta}}In {{bold}}[JustBeforeEach]{{/}}{{magenta}} at: {{bold}}"+cl4.String()+"{{/}}",
			DELIMITER,
			"",
		),
		Entry("when a test has panicked and there is a forwarded panic",
			C(),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
				types.SpecStatePanicked, 2,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4, ForwardedPanic("the panic\nthusly forwarded")),
			),
			DELIMITER,
			"{{magenta}}"+DENOTER+"! [PANICKED] [1.000 seconds]{{/}}",
			"Describe A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{magenta}}{{bold}}Context B [JustBeforeEach]{{/}}",
			"  {{gray}}"+cl3.String()+"{{/}}",
			"    The Test",
			"    {{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{magenta}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{magenta}}In {{bold}}[JustBeforeEach]{{/}}{{magenta}} at: {{bold}}"+cl4.String()+"{{/}}",
			"",
			"  {{magenta}}the panic",
			"  thusly forwarded{{/}}",
			"",
			"  {{magenta}}Full Stack Trace{{/}}",
			"    full-trace",
			"    cl-4",
			DELIMITER,
			"",
		),

		Entry("when a test is interrupted",
			C(),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
				types.SpecStateInterrupted, 2,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4, PR(types.NodeTypeBeforeSuite)),
			),
			DELIMITER,
			"{{orange}}"+DENOTER+"! [INTERRUPTED] [1.000 seconds]{{/}}",
			"Describe A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{orange}}{{bold}}Context B [JustBeforeEach]{{/}}",
			"  {{gray}}"+cl3.String()+"{{/}}",
			"    The Test",
			"    {{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{orange}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{orange}}In {{bold}}[JustBeforeEach]{{/}}{{orange}} at: {{bold}}"+cl4.String()+"{{/}}",
			"",
			REGEX(`  In {{bold}}{{orange}}\[BeforeSuite\]{{/}} \(Node Runtime: 3[\.\d]*s\)`),
			"    {{gray}}cl1.go:37{{/}}",
			DELIMITER,
			"",
		),

		Entry("when a test is aborted",
			C(),
			S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
				types.SpecStateAborted, 2,
				GW("GW-OUTPUT\nIS EMITTED"), STD("STD-OUTPUT\nIS EMITTED"),
				F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4),
			),
			DELIMITER,
			"{{coral}}"+DENOTER+"! [ABORTED] [1.000 seconds]{{/}}",
			"Describe A",
			"{{gray}}"+cl0.String()+"{{/}}",
			"  {{coral}}{{bold}}Context B [JustBeforeEach]{{/}}",
			"  {{gray}}"+cl3.String()+"{{/}}",
			"    The Test",
			"    {{gray}}"+cl2.String()+"{{/}}",
			"",
			"  {{gray}}Begin Captured StdOut/StdErr Output >>{{/}}",
			"    STD-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured StdOut/StdErr Output{{/}}",
			"",
			"  {{gray}}Begin Captured GinkgoWriter Output >>{{/}}",
			"    GW-OUTPUT",
			"    IS EMITTED",
			"  {{gray}}<< End Captured GinkgoWriter Output{{/}}",
			"",
			"  {{coral}}FAILURE MESSAGE",
			"  WITH DETAILS{{/}}",
			"  {{coral}}In {{bold}}[JustBeforeEach]{{/}}{{coral}} at: {{bold}}"+cl4.String()+"{{/}}",
			DELIMITER,
			"",
		))

	DescribeTable("Rendering SuiteDidEnd",
		func(conf types.ReporterConfig, report types.Report, expected ...string) {
			reporter := reporters.NewDefaultReporterUnderTest(conf, buf)
			reporter.SuiteDidEnd(report)
			verifyExpectedOutput(expected)
		},

		Entry("when configured to be succinct",
			C(Succinct),
			types.Report{
				SuiteSucceeded: true,
				RunTime:        time.Minute,
				SpecReports:    types.SpecReports{S()},
			},
			" {{green}}SUCCESS!{{/}} 1m0s ",
		),
		Entry("the suite passes",
			C(),
			types.Report{
				SuiteSucceeded: true,
				PreRunStats:    types.PreRunStats{TotalSpecs: 8, SpecsThatWillRun: 8},
				RunTime:        time.Minute,
				SpecReports: types.SpecReports{
					S(types.NodeTypeBeforeSuite),
					S(types.SpecStatePassed), S(types.SpecStatePassed), S(types.SpecStatePassed),
					S(types.SpecStatePending), S(types.SpecStatePending),
					S(types.SpecStateSkipped), S(types.SpecStateSkipped), S(types.SpecStateSkipped),
					S(types.NodeTypeAfterSuite),
				},
			},
			"",
			"{{green}}{{bold}}Ran 3 of 8 Specs in 60.000 seconds{{/}}",
			"{{green}}{{bold}}SUCCESS!{{/}} -- {{green}}{{bold}}3 Passed{{/}} | {{red}}{{bold}}0 Failed{{/}} | {{yellow}}{{bold}}2 Pending{{/}} | {{cyan}}{{bold}}3 Skipped{{/}}",
			"",
		),
		Entry("the suite passes and has flaky specs",
			C(),
			types.Report{
				SuiteSucceeded: true,
				PreRunStats:    types.PreRunStats{TotalSpecs: 10, SpecsThatWillRun: 8},
				RunTime:        time.Minute,
				SpecReports: types.SpecReports{
					S(types.NodeTypeBeforeSuite),
					S(types.SpecStatePassed), S(types.SpecStatePassed), S(types.SpecStatePassed),
					S(types.SpecStatePassed, 3), S(types.SpecStatePassed, 4), //flakey
					S(types.SpecStatePending), S(types.SpecStatePending),
					S(types.SpecStateSkipped), S(types.SpecStateSkipped), S(types.SpecStateSkipped),
					S(types.NodeTypeAfterSuite),
				},
			},
			"",
			"{{green}}{{bold}}Ran 5 of 10 Specs in 60.000 seconds{{/}}",
			"{{green}}{{bold}}SUCCESS!{{/}} -- {{green}}{{bold}}5 Passed{{/}} | {{red}}{{bold}}0 Failed{{/}} | {{light-yellow}}{{bold}}2 Flaked{{/}} | {{yellow}}{{bold}}2 Pending{{/}} | {{cyan}}{{bold}}3 Skipped{{/}}",
			"",
		),
		Entry("the suite fails with one failed test",
			C(),
			types.Report{
				SuiteSucceeded: false,
				PreRunStats:    types.PreRunStats{TotalSpecs: 11, SpecsThatWillRun: 9},
				RunTime:        time.Minute,
				SpecReports: types.SpecReports{
					S(types.NodeTypeBeforeSuite),
					S(types.SpecStatePassed), S(types.SpecStatePassed), S(types.SpecStatePassed),
					S(types.SpecStatePassed, 3), S(types.SpecStatePassed, 4), //flakey
					S(types.SpecStatePending), S(types.SpecStatePending),
					S(types.SpecStateSkipped), S(types.SpecStateSkipped), S(types.SpecStateSkipped),
					S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2,
						types.SpecStateFailed, 2,
						F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4),
					),
					S(types.NodeTypeAfterSuite),
				},
			},
			"",
			"",
			"{{red}}{{bold}}Summarizing 1 Failure:{{/}}",
			"  {{red}}[FAIL]{{/}} {{/}}Describe A {{gray}}{{red}}{{bold}}Context B [JustBeforeEach]{{/}} {{/}}The Test{{/}}",
			"  {{gray}}cl4.go:144{{/}}",
			"",
			"{{red}}{{bold}}Ran 6 of 11 Specs in 60.000 seconds{{/}}",
			"{{red}}{{bold}}FAIL!{{/}} -- {{green}}{{bold}}5 Passed{{/}} | {{red}}{{bold}}1 Failed{{/}} | {{light-yellow}}{{bold}}2 Flaked{{/}} | {{yellow}}{{bold}}2 Pending{{/}} | {{cyan}}{{bold}}3 Skipped{{/}}",
			"",
		),
		Entry("the suite fails with multiple failed tests",
			C(),
			types.Report{
				SuiteSucceeded: false,
				PreRunStats:    types.PreRunStats{TotalSpecs: 13, SpecsThatWillRun: 9},
				RunTime:        time.Minute,
				SpecReports: types.SpecReports{
					S(types.NodeTypeBeforeSuite),
					S(types.SpecStatePassed), S(types.SpecStatePassed), S(types.SpecStatePassed),
					S(types.SpecStatePassed, 3), S(types.SpecStatePassed, 4), //flakey
					S(types.SpecStatePending), S(types.SpecStatePending),
					S(types.SpecStateSkipped), S(types.SpecStateSkipped), S(types.SpecStateSkipped),
					S(CTS("Describe A", "Context B"), "The Test", CLS(cl0, cl1), cl2, CLabels(Label("cat", "dog"), Label("dog", "fish")), Label("fish", "giraffe"),
						types.SpecStateFailed, 2,
						F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeInContainer, FailureNodeLocation(cl3), types.NodeTypeJustBeforeEach, 1, cl4),
					),
					S(CTS("Describe A"), "The Test", CLS(cl0), cl1,
						types.SpecStatePanicked, 2,
						F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, FailureNodeLocation(cl1), types.NodeTypeIt, cl2),
					),
					S("The Test", cl0,
						types.SpecStateInterrupted, 2,
						F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, FailureNodeLocation(cl0), types.NodeTypeIt, cl1),
					),
					S("The Test", cl0,
						types.SpecStateAborted, 2,
						F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, FailureNodeLocation(cl0), types.NodeTypeIt, cl1),
					),
					S(types.NodeTypeAfterSuite),
				},
			},
			"",
			"",
			"{{red}}{{bold}}Summarizing 4 Failures:{{/}}",
			"  {{red}}[FAIL]{{/}} {{/}}Describe A {{gray}}{{red}}{{bold}}Context B [JustBeforeEach]{{/}} {{/}}The Test{{/}} {{coral}}[cat, dog, fish, giraffe]{{/}}",
			"  {{gray}}"+cl4.String()+"{{/}}",
			"  {{magenta}}[PANICKED!]{{/}} {{/}}Describe A {{gray}}{{magenta}}{{bold}}[It] The Test{{/}}{{/}}",
			"  {{gray}}"+cl2.String()+"{{/}}",
			"  {{orange}}[INTERRUPTED]{{/}} {{/}}{{orange}}{{bold}}[It] The Test{{/}}{{/}}",
			"  {{gray}}"+cl1.String()+"{{/}}",
			"  {{coral}}[ABORTED]{{/}} {{/}}{{coral}}{{bold}}[It] The Test{{/}}{{/}}",
			"  {{gray}}"+cl1.String()+"{{/}}",
			"",
			"{{red}}{{bold}}Ran 9 of 13 Specs in 60.000 seconds{{/}}",
			"{{red}}{{bold}}FAIL!{{/}} -- {{green}}{{bold}}5 Passed{{/}} | {{red}}{{bold}}4 Failed{{/}} | {{light-yellow}}{{bold}}2 Flaked{{/}} | {{yellow}}{{bold}}2 Pending{{/}} | {{cyan}}{{bold}}3 Skipped{{/}}",
			"",
		),
		Entry("the suite fails with failed suite setups",
			C(),
			types.Report{
				SuiteSucceeded: false,
				PreRunStats:    types.PreRunStats{TotalSpecs: 10, SpecsThatWillRun: 5},
				RunTime:        time.Minute,
				SpecReports: types.SpecReports{
					S(types.NodeTypeBeforeSuite, cl0, types.SpecStateFailed, 2,
						F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, FailureNodeLocation(cl0), types.NodeTypeBeforeSuite, 1, cl1),
					),
					S(types.NodeTypeAfterSuite, cl2, types.SpecStateFailed, 2,
						F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, FailureNodeLocation(cl2), types.NodeTypeAfterSuite, 1, cl3),
					),
					S(types.NodeTypeReportAfterSuite, "my report", cl1, types.SpecStateFailed, 2,
						F("FAILURE MESSAGE\nWITH DETAILS", types.FailureNodeIsLeafNode, FailureNodeLocation(cl3), types.NodeTypeReportAfterSuite, 1, cl4),
					),
				},
			},
			"",
			"",
			"{{red}}{{bold}}Summarizing 3 Failures:{{/}}",
			"  {{red}}[FAIL]{{/}} {{/}}{{red}}{{bold}}[BeforeSuite] {{/}}{{/}}",
			"  {{gray}}"+cl1.String()+"{{/}}",
			"  {{red}}[FAIL]{{/}} {{/}}{{red}}{{bold}}[AfterSuite] {{/}}{{/}}",
			"  {{gray}}"+cl3.String()+"{{/}}",
			"  {{red}}[FAIL]{{/}} {{/}}{{red}}{{bold}}[ReportAfterSuite] my report{{/}}{{/}}",
			"  {{gray}}"+cl4.String()+"{{/}}",
			"",
			"{{red}}{{bold}}Ran 0 of 10 Specs in 60.000 seconds{{/}}",
			"{{red}}{{bold}}FAIL!{{/}} -- {{cyan}}{{bold}}A BeforeSuite node failed so all tests were skipped.{{/}}",
			"",
		),

		Entry("when the suite includes a special failure reason",
			C(),
			types.Report{
				SuiteSucceeded:             false,
				SpecialSuiteFailureReasons: []string{"Detected pending specs and --fail-on-pending is set"},
				SuiteConfig:                types.SuiteConfig{FailOnPending: true},
				PreRunStats:                types.PreRunStats{TotalSpecs: 5, SpecsThatWillRun: 3},
				RunTime:                    time.Minute,
				SpecReports: types.SpecReports{
					S(types.SpecStatePassed), S(types.SpecStatePassed), S(types.SpecStatePassed),
					S(types.SpecStatePending), S(types.SpecStatePending),
				},
			},
			"",
			"{{red}}{{bold}}Ran 3 of 5 Specs in 60.000 seconds{{/}}",
			"{{red}}{{bold}}FAIL! - Detected pending specs and --fail-on-pending is set{{/}} -- {{green}}{{bold}}3 Passed{{/}} | {{red}}{{bold}}0 Failed{{/}} | {{yellow}}{{bold}}2 Pending{{/}} | {{cyan}}{{bold}}0 Skipped{{/}}",
			"",
		),
		Entry("when the suite includes multiple special failure reasons",
			C(),
			types.Report{
				SuiteSucceeded:             false,
				SpecialSuiteFailureReasons: []string{"Detected pending specs and --fail-on-pending is set", "Interrupted by Timeout"},
				SuiteConfig:                types.SuiteConfig{FailOnPending: true},
				PreRunStats:                types.PreRunStats{TotalSpecs: 5, SpecsThatWillRun: 3},
				RunTime:                    time.Minute,
				SpecReports: types.SpecReports{
					S(types.SpecStatePassed), S(types.SpecStatePassed), S(types.SpecStatePassed),
					S(types.SpecStatePending), S(types.SpecStatePending),
				},
			},
			"",
			"{{red}}{{bold}}Ran 3 of 5 Specs in 60.000 seconds{{/}}",
			"{{red}}{{bold}}FAIL! - Detected pending specs and --fail-on-pending is set, Interrupted by Timeout{{/}}",
			"{{green}}{{bold}}3 Passed{{/}} | {{red}}{{bold}}0 Failed{{/}} | {{yellow}}{{bold}}2 Pending{{/}} | {{cyan}}{{bold}}0 Skipped{{/}}",
			"",
		),
	)

	DescribeTable("EmitProgressReport",
		func(conf types.ReporterConfig, report types.ProgressReport, expected ...interface{}) {
			reporter := reporters.NewDefaultReporterUnderTest(conf, buf)
			reporter.EmitProgressReport(report)
			verifyRegExOutput(expected)
		},
		//just headers to start
		Entry("With a suite node",
			C(),
			PR(types.NodeTypeBeforeSuite),
			DELIMITER,
			REGEX(`In {{bold}}{{orange}}\[BeforeSuite\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"  {{gray}}"+cl1.String()+"{{/}}",
			DELIMITER,
			""),
		Entry("With a top-level spec",
			C(),
			PR(types.NodeTypeIt, CurrentNodeText("A Top-Level It"), "A Top-Level It"),
			DELIMITER,
			REGEX(`{{bold}}{{orange}}A Top-Level It{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			DELIMITER,
			""),
		Entry("With a spec in containers",
			C(),
			PR(types.NodeTypeIt, CurrentNodeText("My Spec"), "My Spec", []string{"Container A", "Container B", "Container C"}),
			DELIMITER,
			REGEX(`{{/}}Container A {{gray}}Container B {{/}}Container C{{/}} {{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			DELIMITER,
			""),
		Entry("With no current node",
			C(),
			PR("My Spec", []string{"Container A", "Container B", "Container C"}),
			DELIMITER,
			REGEX(`{{/}}Container A {{gray}}Container B {{/}}Container C{{/}} {{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			DELIMITER,
			""),
		Entry("With a current node that is not an It",
			C(),
			PR("My Spec", []string{"Container A", "Container B", "Container C"}, types.NodeTypeBeforeEach),
			DELIMITER,
			REGEX(`{{/}}Container A {{gray}}Container B {{/}}Container C{{/}} {{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[BeforeEach\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			DELIMITER,
			""),
		Entry("With a current node that is not an It, but has text",
			C(),
			PR(types.NodeTypeReportAfterSuite, CurrentNodeText("My Report")),
			DELIMITER,
			REGEX(`In {{bold}}{{orange}}\[ReportAfterSuite\]{{/}} {{bold}}{{orange}}My Report{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"  {{gray}}"+cl1.String()+"{{/}}",
			DELIMITER,
			""),
		Entry("With a current step",
			C(),
			PR(types.NodeTypeIt, CurrentNodeText("My Spec"), "My Spec", []string{"Container A", "Container B", "Container C"}, CurrentStepText("Reticulating Splines")),
			DELIMITER,
			REGEX(`{{/}}Container A {{gray}}Container B {{/}}Container C{{/}} {{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			REGEX(`    At {{bold}}{{orange}}\[By Step\] Reticulating Splines{{/}} \(Step Runtime: 1[\d\.]*s\)`),
			"      {{gray}}"+cl2.String()+"{{/}}",
			DELIMITER,
			""),

		//various goroutines
		Entry("with a spec goroutine",
			C(),
			PR(
				types.NodeTypeIt, CurrentNodeText("My Spec"), "My Spec",
				G(true, "sleeping",
					Fn("F1()", "fileA", 15),
					Fn("F2()", "fileB", 11, true),
					Fn("F3()", "fileC", 9),
				),
			),

			DELIMITER,
			REGEX(`{{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{bold}}{{underline}}Spec Goroutine{{/}}",
			"  {{orange}}goroutine 17 [sleeping]{{/}}",
			"    {{gray}}F1(){{/}}",
			"      {{gray}}fileA:15{{/}}",
			"  {{orange}}{{bold}}> F2(){{/}}",
			"      {{orange}}{{bold}}fileB:11{{/}}",
			"    {{gray}}F3(){{/}}",
			"      {{gray}}fileC:9{{/}}",
			DELIMITER,
			""),

		Entry("with highlighted goroutines",
			C(),
			PR(
				types.NodeTypeIt, CurrentNodeText("My Spec"), "My Spec",
				G(false, "sleeping",
					Fn("F1()", "fileA", 15),
					Fn("F2()", "fileB", 11, true),
					Fn("F3()", "fileC", 9),
				),
			),

			DELIMITER,
			REGEX(`{{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{bold}}{{underline}}Goroutines of Interest{{/}}",
			"  {{orange}}goroutine 17 [sleeping]{{/}}",
			"    {{gray}}F1(){{/}}",
			"      {{gray}}fileA:15{{/}}",
			"  {{orange}}{{bold}}> F2(){{/}}",
			"      {{orange}}{{bold}}fileB:11{{/}}",
			"    {{gray}}F3(){{/}}",
			"      {{gray}}fileC:9{{/}}",
			DELIMITER,
			""),

		Entry("with other goroutines",
			C(),
			PR(
				types.NodeTypeIt, CurrentNodeText("My Spec"), "My Spec",
				G(false, "sleeping",
					Fn("F1()", "fileA", 15),
					Fn("F2()", "fileB", 11),
					Fn("F3()", "fileC", 9),
				),
			),

			DELIMITER,
			REGEX(`{{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{gray}}{{bold}}{{underline}}Other Goroutines{{/}}",
			"  {{gray}}goroutine 17 [sleeping]{{/}}",
			"    {{gray}}F1(){{/}}",
			"      {{gray}}fileA:15{{/}}",
			"    {{gray}}F2(){{/}}",
			"      {{gray}}fileB:11{{/}}",
			"    {{gray}}F3(){{/}}",
			"      {{gray}}fileC:9{{/}}",
			DELIMITER,
			""),

		//fetching source code
		Entry("when source code is found",
			C(),
			PR(
				types.NodeTypeIt, CurrentNodeText("My Spec"), "My Spec",
				G(true, "sleeping",
					Fn("F1()", "fileA", 15),
					Fn("F2()", "reporters_suite_test.go", 21, true),
					Fn("F3()", "fileC", 9),
				),
			),

			DELIMITER,
			REGEX(`{{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{bold}}{{underline}}Spec Goroutine{{/}}",
			"  {{orange}}goroutine 17 [sleeping]{{/}}",
			"    {{gray}}F1(){{/}}",
			"      {{gray}}fileA:15{{/}}",
			"  {{orange}}{{bold}}> F2(){{/}}",
			"      {{orange}}{{bold}}reporters_suite_test.go:21{{/}}",
			"        | func FixtureFunction() {",
			"        | \ta := 0",
			"        {{bold}}{{orange}}> \tfor a < 100 {{{/}}",
			"        | \t\tfmt.Println(a)",
			"        | \t\tfmt.Println(a + 1)",
			"    {{gray}}F3(){{/}}",
			"      {{gray}}fileC:9{{/}}",
			DELIMITER,
			""),

		Entry("correcting source code indentation",
			C(),
			PR(
				types.NodeTypeIt, CurrentNodeText("My Spec"), "My Spec",
				G(true, "sleeping",
					Fn("F1()", "fileA", 15),
					Fn("F2()", "reporters_suite_test.go", 26, true),
					Fn("F3()", "fileC", 9),
				),
			),

			DELIMITER,
			REGEX(`{{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			"",
			"  {{bold}}{{underline}}Spec Goroutine{{/}}",
			"  {{orange}}goroutine 17 [sleeping]{{/}}",
			"    {{gray}}F1(){{/}}",
			"      {{gray}}fileA:15{{/}}",
			"  {{orange}}{{bold}}> F2(){{/}}",
			"      {{orange}}{{bold}}reporters_suite_test.go:26{{/}}",
			"        | fmt.Println(a + 3)",
			"        | fmt.Println(a + 4)",
			"        {{bold}}{{orange}}> fmt.Println(a + 5){{/}}",
			"        | ",
			"        | fmt.Println(a + 6)",
			"    {{gray}}F3(){{/}}",
			"      {{gray}}fileC:9{{/}}",
			DELIMITER,
			""),

		Entry("when running in parallel",
			C(),
			PR(
				true, 3,
				types.NodeTypeIt, CurrentNodeText("My Spec"), "My Spec",
			),

			DELIMITER,
			"{{coral}}Progress Report for Ginkgo Process #{{bold}}3{{/}}",
			REGEX(`{{bold}}{{orange}}My Spec{{/}} \(Spec Runtime: 5[\d\.]*s\)`),
			"  {{gray}}"+cl0.String()+"{{/}}",
			REGEX(`  In {{bold}}{{orange}}\[It\]{{/}} \(Node Runtime: 3[\d\.]*s\)`),
			"    {{gray}}"+cl1.String()+"{{/}}",
			DELIMITER,
			""),
	)

})
