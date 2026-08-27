package buildtool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

const localAggregateIndexName = "aggregate-index.json"

var (
	localAggregateCorrelationPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	localAggregateSHApattern         = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
)

// LocalPackageTargetStatus is the optional Docker aggregate's target state.
type LocalPackageTargetStatus string

const (
	LocalPackageTargetBuilding LocalPackageTargetStatus = "building"
	LocalPackageTargetEligible LocalPackageTargetStatus = "eligible"
	LocalPackageTargetFailed   LocalPackageTargetStatus = "failed"
)

// LocalPackageTargetRecord describes one locally built Docker target.
type LocalPackageTargetRecord struct {
	Target      Target
	SourceSHA   string
	Status      LocalPackageTargetStatus
	ArchiveName string
	Checksum    string
	Failure     error
}

// LocalVerifiedArtifact is the verified identity retained by local packaging.
type LocalVerifiedArtifact interface {
	Target() Target
	SourceRevision() string
	ArchiveName() string
	Checksum() string
}

// LocalPackageResult contains the optional local Docker aggregate outputs.
type LocalPackageResult struct {
	CorrelationID    string
	SourceSHA        string
	Records          []LocalPackageTargetRecord
	OutputDirectory  string
	DarwinBundlePath string
	Artifacts        []LocalVerifiedArtifact
}

func localDockerTargets() []Target {
	return []Target{
		{goos: goosWindows, goarch: goarchAMD64},
		{goos: goosWindows, goarch: goarchARM64},
		{goos: goosLinux, goarch: goarchAMD64},
		{goos: goosLinux, goarch: goarchARM64},
	}
}

func cloneLocalPackageRecords(records []LocalPackageTargetRecord) []LocalPackageTargetRecord {
	return append([]LocalPackageTargetRecord(nil), records...)
}

func validateLocalPackageRecords(records []LocalPackageTargetRecord, sourceSHA string) error {
	if len(records) != len(localDockerTargets()) {
		return fmt.Errorf("local aggregate requires exactly four target records, got %d", len(records))
	}
	seen := make(map[string]struct{}, len(records))
	var errs []error
	for _, record := range records {
		name := record.Target.String()
		if _, exists := seen[name]; exists {
			errs = append(errs, fmt.Errorf("local aggregate contains duplicate target %s", name))
			continue
		}
		seen[name] = struct{}{}
		if record.SourceSHA != sourceSHA {
			errs = append(errs, fmt.Errorf("target %s source revision mismatch", name))
		}
		if record.ArchiveName != record.Target.ArchiveName() {
			errs = append(errs, fmt.Errorf("target %s archive name mismatch", name))
		}
		if record.Status != LocalPackageTargetEligible {
			cause := record.Failure
			if cause == nil {
				cause = fmt.Errorf("status is %q", record.Status)
			}
			errs = append(errs, fmt.Errorf("target %s failed: %w", name, cause))
		}
	}
	for _, target := range localDockerTargets() {
		if _, exists := seen[target.String()]; !exists {
			errs = append(errs, fmt.Errorf("local aggregate is missing target %s", target))
		}
	}
	return errors.Join(errs...)
}

func validateLocalPackageArtifacts(artifacts []LocalVerifiedArtifact, records []LocalPackageTargetRecord, sourceSHA string) error {
	if len(artifacts) != len(localDockerTargets()) {
		return fmt.Errorf("local aggregate requires exactly four verified artifacts, got %d", len(artifacts))
	}
	recordsByTarget := make(map[string]LocalPackageTargetRecord, len(records))
	for _, record := range records {
		recordsByTarget[record.Target.String()] = record
	}
	seen := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		if artifact == nil {
			return errors.New("local aggregate verifier returned a nil artifact")
		}
		name := artifact.Target().String()
		if _, exists := seen[name]; exists {
			return fmt.Errorf("local aggregate verifier returned duplicate target %s", name)
		}
		seen[name] = struct{}{}
		record, exists := recordsByTarget[name]
		if !exists || artifact.SourceRevision() != sourceSHA || artifact.ArchiveName() != record.ArchiveName || artifact.Checksum() != record.Checksum {
			return fmt.Errorf("verified local artifact does not match target record %s", name)
		}
	}
	return nil
}

type localAggregateIndexDocument struct {
	SchemaVersion  int                           `json:"schemaVersion"`
	CorrelationID  string                        `json:"correlationID"`
	SourceRevision string                        `json:"sourceRevision"`
	Artifacts      []localAggregateIndexArtifact `json:"artifacts"`
}

type localAggregateIndexArtifact struct {
	Target      string `json:"target"`
	ArchiveName string `json:"archiveName"`
	Checksum    string `json:"checksum"`
}

type directoryLocalAggregateVerifier struct{}

func (directoryLocalAggregateVerifier) Verify(ctx context.Context, directory string) ([]LocalVerifiedArtifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read local aggregate directory: %w", err)
	}
	expected := make(map[string]struct{}, len(localDockerTargets())*2+1)
	expected[localAggregateIndexName] = struct{}{}
	for _, target := range localDockerTargets() {
		expected[target.ArchiveName()] = struct{}{}
		expected[target.ArchiveName()+".sha256"] = struct{}{}
	}
	if len(entries) != len(expected) {
		return nil, fmt.Errorf("local aggregate requires exactly %d files, got %d", len(expected), len(entries))
	}
	for _, entry := range entries {
		if _, found := expected[entry.Name()]; !found || entry.IsDir() {
			return nil, fmt.Errorf("local aggregate contains unexpected entry %q", entry.Name())
		}
	}
	artifacts := make([]LocalVerifiedArtifact, 0, len(localDockerTargets()))
	for _, target := range localDockerTargets() {
		archive := filepath.Join(directory, target.ArchiveName())
		verified, err := VerifyArtifact(ctx, archive, archive+".sha256", target)
		if err != nil {
			return nil, fmt.Errorf("verify %s local artifact: %w", target, err)
		}
		artifacts = append(artifacts, verified)
	}
	if err := verifyLocalAggregateIndex(ctx, filepath.Join(directory, localAggregateIndexName), artifacts); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func verifyLocalAggregateIndex(ctx context.Context, indexPath string, artifacts []LocalVerifiedArtifact) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	contents, err := readLimitedFile(ctx, indexPath, maxManifestSize)
	if err != nil {
		return fmt.Errorf("read local aggregate index: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var index localAggregateIndexDocument
	if err := decoder.Decode(&index); err != nil {
		return fmt.Errorf("decode local aggregate index: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("local aggregate index contains trailing JSON")
	}
	if index.SchemaVersion != 1 || !localAggregateCorrelationPattern.MatchString(index.CorrelationID) || !localAggregateSHApattern.MatchString(index.SourceRevision) {
		return errors.New("local aggregate index identity is invalid")
	}
	if len(index.Artifacts) != len(artifacts) {
		return fmt.Errorf("local aggregate index requires exactly four artifacts, got %d", len(index.Artifacts))
	}
	verified := make(map[string]LocalVerifiedArtifact, len(artifacts))
	for _, artifact := range artifacts {
		verified[artifact.Target().String()] = artifact
	}
	for _, indexed := range index.Artifacts {
		artifact, exists := verified[indexed.Target]
		if !exists || artifact.SourceRevision() != index.SourceRevision || artifact.ArchiveName() != indexed.ArchiveName || artifact.Checksum() != indexed.Checksum {
			return fmt.Errorf("local aggregate index does not match target %s", indexed.Target)
		}
		delete(verified, indexed.Target)
	}
	if len(verified) != 0 {
		return errors.New("local aggregate index is missing a target")
	}
	return nil
}
