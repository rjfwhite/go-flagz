// Copyright 2016 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package configmap_test

import (
	"testing"
	"time"

	flag "github.com/spf13/pflag"

	"io/ioutil"
	"os/exec"

	"path"

	"os"

	"github.com/mwitkow/go-flagz"
	"github.com/mwitkow/go-flagz/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	firstGoodDir  = "..9989_09_09_07_32_32.099817316"
	secondGoodDir = "..9289_09_10_03_32_32.039823124"
	badStaticDir  = "..1289_09_10_03_32_32.039823124"
)

type updaterTestSuite struct {
	suite.Suite

	tempDir string

	flagSet   *flag.FlagSet
	staticInt *int32
	dynInt    *flagz.DynInt64Value

	updater *configmap.Updater
}

func (s *updaterTestSuite) SetupTest() {
	var err error
	s.tempDir, err = ioutil.TempDir("/tmp", "updater_test")
	require.NoError(s.T(), err, "failed creating temp directory for testing")
	s.copyTestDataToDir()
	s.linkDataDirTo(firstGoodDir)

	s.flagSet = flag.NewFlagSet("updater_test", flag.ContinueOnError)
	s.dynInt = flagz.DynInt64(s.flagSet, "some_dynint", 1, "dynamic int for testing")
	s.staticInt = s.flagSet.Int32("some_int", 1, "static int for testing")

	s.updater, err = configmap.New(s.flagSet, path.Join(s.tempDir, "testdata"), &testingLog{T: s.T()})
	require.NoError(s.T(), err, "creating a config map must not fail")
}

// Tear down the updater
func (s *updaterTestSuite) TearDownTest() {
	require.NoError(s.T(), os.RemoveAll(s.tempDir), "clearing up the test dir must not fail")

	s.updater.Stop()
	time.Sleep(100 * time.Millisecond)
}

func (s *updaterTestSuite) copyTestDataToDir() {
	copyCmd := exec.Command("cp", "--archive", "testdata", s.tempDir)
	require.NoError(s.T(), copyCmd.Run(), "copying testdata directory to tempdir must not fail")
}

func (s *updaterTestSuite) linkDataDirTo(newDataDir string) {
	copyCmd := exec.Command("ln", "--symbolic", "--no-dereference", "--force",
		path.Join(s.tempDir, "testdata", newDataDir),
		path.Join(s.tempDir, "testdata", "..data"))
	require.NoError(s.T(), copyCmd.Run(), "relinking ..data in tempdir tempdir must not fail")
}

func (s *updaterTestSuite) TestInitializeFailsOnBadFormedFlag() {
	s.linkDataDirTo(badStaticDir)
	require.Error(s.T(), s.updater.Initialize(), "the updater initialize should return error on bad flags")
}

func (s *updaterTestSuite) TestInitializeSetsValues() {
	require.NoError(s.T(), s.updater.Initialize(), "the updater initialize should not return errors on good flags")
	assert.EqualValues(s.T(), *s.staticInt, 1234, "staticInt should be some_int from first directory")
	assert.EqualValues(s.T(), s.dynInt.Get(), 10001, "staticInt should be some_int from first directory")
}

func (s *updaterTestSuite) TestDynamicUpdatesPropagate() {
	require.NoError(s.T(), s.updater.Initialize(), "the updater initialize should not return errors on good flags")
	require.NoError(s.T(), s.updater.Start(), "updater start should not return an error")
	s.linkDataDirTo(secondGoodDir)
	eventually(s.T(), 1*time.Second,
		assert.ObjectsAreEqualValues, 20002,
		func() interface{} { return s.dynInt.Get() },
		"some_dynint value should change to the value from secondGoodDir")
}

func TestUpdaterSuite(t *testing.T) {
	suite.Run(t, &updaterTestSuite{})
}

type assertFunc func(expected, actual interface{}) bool
type getter func() interface{}

// eventually tries a given Assert function 5 times over the period of time.
func eventually(t *testing.T, duration time.Duration,
	af assertFunc, expected interface{}, actual getter, msgFmt string, msgArgs ...interface{}) {
	increment := duration / 5
	for i := 0; i < 5; i++ {
		time.Sleep(increment)
		if af(expected, actual()) {
			return
		}
	}
	t.Fatalf(msgFmt, msgArgs...)
}

// Abstraction that allows us to pass the *testing.T as a logger to the updater.
type testingLog struct {
	T *testing.T
}

func (tl *testingLog) Printf(format string, v ...interface{}) {
	tl.T.Logf(format+"\n", v...)
}
