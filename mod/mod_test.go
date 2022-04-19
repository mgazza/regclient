package mod

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/regclient/regclient"
	"github.com/regclient/regclient/internal/rwfs"
	"github.com/regclient/regclient/types/platform"
	"github.com/regclient/regclient/types/ref"
)

func TestMod(t *testing.T) {
	ctx := context.Background()
	// copy testdata images into memory
	fsOS := rwfs.OSNew("")
	fsMem := rwfs.MemNew()
	err := rwfs.CopyRecursive(fsOS, "../testdata", fsMem, ".")
	if err != nil {
		t.Errorf("failed to setup memfs copy: %v", err)
		return
	}
	tTime, err := time.Parse(time.RFC3339, "2020-01-01T00:00:00Z")
	if err != nil {
		t.Errorf("failed to parse test time: %v", err)
	}
	bDig := digest.FromString("digest for base image")
	bRef, err := ref.New("base:latest")
	if err != nil {
		t.Errorf("failed to parse base image: %v", err)
	}
	// create regclient
	rc := regclient.New(regclient.WithFS(fsMem))

	r3, err := ref.New("ocidir://testrepo:v3")
	if err != nil {
		t.Errorf("failed to parse ref: %v", err)
	}
	m3, err := rc.ManifestGet(ctx, r3)
	if err != nil {
		t.Errorf("failed to retrieve v3 ref: %v", err)
	}
	pAMD, err := platform.Parse("linux/amd64")
	if err != nil {
		t.Errorf("failed to parse platform: %v", err)
	}
	m3DescAmd, err := m3.GetPlatformDesc(&pAMD)
	if err != nil {
		t.Errorf("failed to get amd64 descriptor: %v", err)
	}
	r3amd, err := ref.New(fmt.Sprintf("ocidir://testrepo@%s", m3DescAmd.Digest.String()))
	if err != nil {
		t.Errorf("failed to parse platform specific descriptor: %v", err)
	}

	// define tests
	tests := []struct {
		name     string
		opts     []Opts
		ref      string
		wantErr  error
		wantSame bool // if the resulting image should be unchanged
	}{
		{
			name: "To OCI",
			opts: []Opts{
				WithManifestToOCI(),
			},
			ref:      "ocidir://testrepo:v1",
			wantSame: true,
		},
		{
			name: "Add Annotation",
			opts: []Opts{
				WithAnnotation("test", "hello"),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Delete Annotation",
			opts: []Opts{
				WithAnnotation("org.example.version", ""),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Add Base Annotations",
			opts: []Opts{
				WithAnnotationOCIBase(bRef, bDig),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Add Label",
			opts: []Opts{
				WithLabel("test", "hello"),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Label to Annotation",
			opts: []Opts{
				WithLabelToAnnotation(),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Time",
			opts: []Opts{
				WithConfigTimestampFromLabel("org.opencontainers.image.created"),
			},
			ref:     "ocidir://testrepo:v1",
			wantErr: fmt.Errorf("label not found: org.opencontainers.image.created"),
		},
		{
			name: "Config Time",
			opts: []Opts{
				WithConfigTimestampMax(tTime),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Config Time Unchanged",
			opts: []Opts{
				WithConfigTimestampMax(time.Now()),
			},
			ref:      "ocidir://testrepo:v1",
			wantSame: true,
		},
		{
			name: "Expose port",
			opts: []Opts{
				WithExposeAdd("8080"),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Expose port delete unchanged",
			opts: []Opts{
				WithExposeRm("8080"),
			},
			ref:      "ocidir://testrepo:v1",
			wantSame: true,
		},
		{
			name: "Layer Timestamp Missing Label",
			opts: []Opts{
				WithLayerTimestampFromLabel("missing"),
			},
			ref:     "ocidir://testrepo:v1",
			wantErr: fmt.Errorf("label not found: missing"),
		},
		{
			name: "Layer Timestamp",
			opts: []Opts{
				WithLayerTimestampMax(tTime),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Layer Timestamp Unchanged",
			opts: []Opts{
				WithLayerTimestampMax(time.Now()),
			},
			ref:      "ocidir://testrepo:v1",
			wantSame: true,
		},
		{
			name: "Layer Trim File",
			opts: []Opts{
				WithLayerStripFile("/layer2"),
			},
			ref: "ocidir://testrepo:v3",
		},
		{
			name: "Layer Trim By Created RE",
			opts: []Opts{
				WithLayerRmCreatedBy(*regexp.MustCompile("^COPY layer2.txt /layer2")),
			},
			ref: "ocidir://testrepo:v3",
		},
		{
			name: "Layer Remove by index from Index",
			opts: []Opts{
				WithLayerRmIndex(1),
			},
			ref:     "ocidir://testrepo:v3",
			wantErr: fmt.Errorf("remove layer by index requires v2 image manifest"),
		},
		{
			name: "Layer Remove by index",
			opts: []Opts{
				WithLayerRmIndex(1),
			},
			ref: r3amd.CommonName(),
		},
		{
			name: "Layer Remove by index missing",
			opts: []Opts{
				WithLayerRmIndex(10),
			},
			ref:     r3amd.CommonName(),
			wantErr: fmt.Errorf("layer not found"),
		},
		{
			name: "Add volume",
			opts: []Opts{
				WithVolumeAdd("/new"),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Add volume again",
			opts: []Opts{
				WithVolumeAdd("/volume"),
			},
			ref:      "ocidir://testrepo:v1",
			wantSame: true,
		},
		{
			name: "Rm volume",
			opts: []Opts{
				WithVolumeRm("/volume"),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Rm volume missing",
			opts: []Opts{
				WithVolumeRm("/test"),
			},
			ref:      "ocidir://testrepo:v1",
			wantSame: true,
		},
		{
			name: "Data field",
			opts: []Opts{
				WithData(2048),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Build arg rm",
			opts: []Opts{
				WithBuildArgRm("arg_label", regexp.MustCompile("arg_for_[a-z]*")),
			},
			ref: "ocidir://testrepo:v1",
		},
		{
			name: "Build arg with value rm",
			opts: []Opts{
				WithBuildArgRm("arg_label", regexp.MustCompile("arg_for_[a-z]*")),
			},
			ref: "ocidir://testrepo:v2",
		},
		{
			name: "Build arg missing",
			opts: []Opts{
				WithBuildArgRm("no_such_arg", regexp.MustCompile("no_such_value")),
			},
			ref:      "ocidir://testrepo:v1",
			wantSame: true,
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ref.New(tt.ref)
			if err != nil {
				t.Errorf("failed creating ref: %v", err)
				return
			}
			// run mod with opts
			rMod, err := Apply(ctx, rc, r, tt.opts...)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("ModImage did not fail")
				} else if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("unexpected error, wanted %v, received %v", tt.wantErr, err)
				}
				return
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantSame {
				if r.Digest != rMod.Digest {
					t.Errorf("digest changed")
				}
			} else {
				if r.Digest == rMod.Digest {
					t.Errorf("digest did not change")
				}
			}
		})
	}
}