package brig

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/nlsantos/brig/writ"
	"github.com/stretchr/testify/assert"
)

func TestParseOverrideFeatureInstallOrderStandalone(t *testing.T) {
	// Silence slog output for the duration of the run
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	// Config composition is done manually to bypass set up and
	// constraints we don't really need nor want

	dcParser, err := writ.NewDevcontainerParser(filepath.Join("testdata", "features-standalone", "devcontainer.json"))
	assert.Nil(t, err)
	assert.Nil(t, dcParser.Validate())
	assert.Nil(t, dcParser.Parse())

	cmd := Command{featureParsersLookup: make(map[string]*writ.DevcontainerFeatureParser)}
	for _, feature := range []string{"alpha", "beta", "gamma", "delta"} {
		p, err := writ.NewDevcontainerFeatureParser(filepath.Join("testdata", "features-standalone", fmt.Sprintf("%s.json", feature)), nil)
		assert.Nil(t, err)
		assert.Nil(t, p.Validate())
		assert.Nil(t, p.Parse())

		cmd.featureParsersLookup[fmt.Sprintf("./%s", feature)] = p
	}

	installDAG, err := cmd.BuildFeaturesInstallationGraph(&dcParser.Config.OverrideFeatureInstallOrder)
	assert.Nil(t, err)

	featureRoots := []string{}
	roots := installDAG.GetRoots()
	for len(roots) > 0 {
		for featureID := range roots {
			featureRoots = append(featureRoots, featureID)
			installDAG.DeleteVertex(featureID)
		}
		roots = installDAG.GetRoots()
	}
	assert.EqualValues(t, dcParser.Config.OverrideFeatureInstallOrder, featureRoots)
}
