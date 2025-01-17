package regclient

import (
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/olareg/olareg"
	oConfig "github.com/olareg/olareg/config"
	"github.com/sirupsen/logrus"

	"github.com/regclient/regclient/config"
	"github.com/regclient/regclient/internal/copy"
	"github.com/regclient/regclient/types/ref"
)

func TestTag(t *testing.T) {
	t.Parallel()
	existingRepo := "testrepo"
	existingTag := "v2"
	ctx := context.Background()
	falseV := false
	regRWHandler := olareg.New(oConfig.Config{
		Storage: oConfig.ConfigStorage{
			StoreType: oConfig.StoreMem,
			RootDir:   "./testdata",
		},
	})
	regROHandler := olareg.New(oConfig.Config{
		Storage: oConfig.ConfigStorage{
			StoreType: oConfig.StoreMem,
			RootDir:   "./testdata",
		},
		API: oConfig.ConfigAPI{
			DeleteEnabled: &falseV,
		},
	})
	// TODO: test with a registry without the delete APIs
	tsRW := httptest.NewServer(regRWHandler)
	tsRWURL, _ := url.Parse(tsRW.URL)
	tsRWHost := tsRWURL.Host
	tsRO := httptest.NewServer(regROHandler)
	tsROURL, _ := url.Parse(tsRO.URL)
	tsROHost := tsROURL.Host
	t.Cleanup(func() {
		tsRW.Close()
		tsRO.Close()
		_ = regRWHandler.Close()
		_ = regROHandler.Close()
	})
	rcHosts := []config.Host{
		{
			Name:      tsRWHost,
			Hostname:  tsRWHost,
			TLS:       config.TLSDisabled,
			ReqPerSec: 1000,
		},
		{
			Name:      tsROHost,
			Hostname:  tsROHost,
			TLS:       config.TLSDisabled,
			ReqPerSec: 1000,
		},
	}
	log := &logrus.Logger{
		Out:       os.Stderr,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.WarnLevel,
	}
	delayInit, _ := time.ParseDuration("0.05s")
	delayMax, _ := time.ParseDuration("0.10s")
	rc := New(
		WithConfigHost(rcHosts...),
		WithLog(log),
		WithRetryDelay(delayInit, delayMax),
	)
	tempDir := t.TempDir()
	err := copy.Copy(tempDir+"/"+existingRepo, "./testdata/"+existingRepo)
	if err != nil {
		t.Errorf("failed to copy %s to tempDir: %v", existingRepo, err)
		return
	}
	tt := []struct {
		name           string
		repo           string
		deleteDisabled bool
	}{
		{
			name: "reg RW",
			repo: tsRWHost + "/" + existingRepo,
		},
		{
			name:           "reg RO",
			repo:           tsROHost + "/" + existingRepo,
			deleteDisabled: true,
		},
		{
			name: "ocidir",
			repo: "ocidir://" + tempDir + "/" + existingRepo,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ref.New(tc.repo)
			if err != nil {
				t.Errorf("failed to parse ref %s: %v", tc.repo, err)
				return
			}
			tl, err := rc.TagList(ctx, r)
			if err != nil {
				t.Errorf("failed to list tags: %v", err)
				return
			}
			if len(tl.Tags) == 0 {
				t.Errorf("failed to get tags: %v", tl)
				return
			}
			rDel, err := ref.New(tc.repo + ":" + existingTag)
			if err != nil {
				t.Errorf("failed to parse ref %s: %v", tc.repo+":"+existingTag, err)
				return
			}
			err = rc.TagDelete(ctx, rDel)
			if tc.deleteDisabled {
				if err == nil {
					t.Errorf("delete succeeded on a read-only repo")
				}
			} else {
				if err != nil {
					t.Errorf("failed to delete tag: %v", err)
				}
			}
		})
	}
}
