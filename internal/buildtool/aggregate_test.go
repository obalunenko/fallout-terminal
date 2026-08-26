package buildtool

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregateRunCorrelatesDispatchDiscoveryAndExactSuccessfulMatrix(t *testing.T) {
	t.Parallel()

	const (
		ref           = "refs/heads/release"
		sourceSHA     = "0123456789abcdef0123456789abcdef01234567"
		correlationID = "package-all-01234567-contract"
	)
	output := filepath.Join(t.TempDir(), "dist")
	records := eligibleAggregateRecords(t, sourceSHA)
	workflow := &fakeAggregateWorkflow{
		resolved: AggregateRevision{Repository: "vaulttec/fallout-terminal", Ref: ref, SourceSHA: sourceSHA},
		found: AggregateRun{
			ID:            42,
			URL:           "https://example.invalid/actions/runs/42",
			CorrelationID: correlationID,
			SourceSHA:     sourceSHA,
		},
		completed: AggregateRun{
			ID:            42,
			URL:           "https://example.invalid/actions/runs/42",
			CorrelationID: correlationID,
			SourceSHA:     sourceSHA,
			Records:       records,
		},
	}
	workflow.download = writeAggregateDownloadFixtures
	verifier := &fakeAggregateVerifier{artifacts: verifiedAggregateArtifacts(records)}
	reported := make([]AggregateTargetRecord, 0, len(records))

	result, err := RunAggregate(t.Context(), AggregateRequest{
		Ref:             ref,
		OutputDirectory: output,
	}, AggregateDependencies{
		Workflow:         workflow,
		Verifier:         verifier,
		NewCorrelationID: func() string { return correlationID },
		Report: func(record AggregateTargetRecord) {
			reported = append(reported, record)
		},
	})
	require.NoError(t, err)

	require.Len(t, workflow.dispatched, 1)
	assert.Equal(t, correlationID, workflow.dispatched[0].CorrelationID)
	assert.Equal(t, ref, workflow.dispatched[0].Revision.Ref)
	assert.Equal(t, sourceSHA, workflow.dispatched[0].Revision.SourceSHA)
	assert.Equal(t, []string{correlationID}, workflow.discoveryIDs)
	assert.Equal(t, correlationID, result.CorrelationID)
	assert.Equal(t, sourceSHA, result.SourceSHA)
	assert.Equal(t, workflow.completed.URL, result.RunURL)
	assert.Equal(t, aggregateRecordTargets(records), aggregateRecordTargets(result.Records))
	assert.Equal(t, aggregateRecordTargets(records), aggregateRecordTargets(reported))
	assert.Equal(t, output, result.OutputDirectory)
	assert.Len(t, result.Artifacts, 4)
	assert.DirExists(t, output)
}

func TestGitHubRepositoryFromRemote(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		remote string
		want   string
	}{
		{name: "SSH", remote: "git@github.com:vaulttec/fallout-terminal.git", want: "vaulttec/fallout-terminal"},
		{name: "HTTPS", remote: "https://github.com/vaulttec/fallout-terminal.git", want: "vaulttec/fallout-terminal"},
		{name: "SSH URL", remote: "ssh://git@github.com/vaulttec/fallout-terminal.git", want: "vaulttec/fallout-terminal"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := githubRepositoryFromRemote(test.remote)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}

	for _, remote := range []string{"", "git@example.com:vaulttec/fallout-terminal.git", "https://github.com/too/many/parts.git"} {
		_, err := githubRepositoryFromRemote(remote)
		require.Error(t, err)
	}
}

func TestAggregateRunRejectsWrongCorrelationBeforeWaitingOrDownloading(t *testing.T) {
	t.Parallel()

	const correlationID = "expected-correlation"
	workflow := &fakeAggregateWorkflow{
		resolved: AggregateRevision{Repository: "vaulttec/fallout-terminal", Ref: "main", SourceSHA: "source-sha"},
		found: AggregateRun{
			ID:            17,
			CorrelationID: "unrelated-run",
			SourceSHA:     "source-sha",
		},
	}

	_, err := RunAggregate(t.Context(), AggregateRequest{
		Ref:             "main",
		OutputDirectory: filepath.Join(t.TempDir(), "dist"),
	}, AggregateDependencies{
		Workflow:         workflow,
		Verifier:         &fakeAggregateVerifier{},
		NewCorrelationID: func() string { return correlationID },
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, correlationID)
	assert.ErrorContains(t, err, "unrelated-run")
	assert.False(t, workflow.waitCalled)
	assert.False(t, workflow.downloadCalled)
}

func TestAggregateRunRejectsWorkflowRunForDifferentSourceRevision(t *testing.T) {
	t.Parallel()

	workflow := successfulAggregateWorkflow(t, "resolved-source", nil)
	workflow.found.SourceSHA = "unrelated-source"

	_, err := RunAggregate(t.Context(), AggregateRequest{
		Ref:             "main",
		OutputDirectory: filepath.Join(t.TempDir(), "dist"),
	}, successfulAggregateDependencies(workflow, nil))
	require.Error(t, err)
	assert.ErrorContains(t, err, "resolved-source")
	assert.ErrorContains(t, err, "unrelated-source")
	assert.False(t, workflow.waitCalled)
	assert.False(t, workflow.downloadCalled)
}

func TestAggregateRunRequiresExactFourTargetsAndOneSourceSHA(t *testing.T) {
	t.Parallel()

	const sourceSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name   string
		mutate func([]AggregateTargetRecord) []AggregateTargetRecord
	}{
		{
			name: "missing target",
			mutate: func(records []AggregateTargetRecord) []AggregateTargetRecord {
				return records[:len(records)-1]
			},
		},
		{
			name: "duplicate target",
			mutate: func(records []AggregateTargetRecord) []AggregateTargetRecord {
				records[len(records)-1].Target = records[0].Target
				return records
			},
		},
		{
			name: "mismatched source revision",
			mutate: func(records []AggregateTargetRecord) []AggregateTargetRecord {
				records[2].SourceSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
				return records
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			workflow := successfulAggregateWorkflow(t, sourceSHA, test.mutate(eligibleAggregateRecords(t, sourceSHA)))
			_, err := RunAggregate(t.Context(), AggregateRequest{
				Ref:             "main",
				OutputDirectory: filepath.Join(t.TempDir(), "dist"),
			}, successfulAggregateDependencies(workflow, nil))
			require.Error(t, err)
			assert.False(t, workflow.downloadCalled, "an invalid workflow matrix must not be downloaded")
		})
	}
}

func TestAggregateRunRejectsDownloadedMetadataOutsideExactTargetSourceJoin(t *testing.T) {
	t.Parallel()

	const sourceSHA = "cccccccccccccccccccccccccccccccccccccccc"
	tests := []struct {
		name   string
		mutate func([]VerifiedArtifact) []VerifiedArtifact
	}{
		{
			name: "duplicate verified target",
			mutate: func(artifacts []VerifiedArtifact) []VerifiedArtifact {
				first := artifacts[0]
				artifacts[len(artifacts)-1] = fakeVerifiedArtifact{
					target:         first.Target(),
					sourceRevision: first.SourceRevision(),
					archiveName:    "duplicate-target.zip",
					checksum:       "duplicate-checksum",
				}
				return artifacts
			},
		},
		{
			name: "mismatched verified source revision",
			mutate: func(artifacts []VerifiedArtifact) []VerifiedArtifact {
				artifact := artifacts[1]
				artifacts[1] = fakeVerifiedArtifact{
					target:         artifact.Target(),
					sourceRevision: "dddddddddddddddddddddddddddddddddddddddd",
					archiveName:    artifact.ArchiveName(),
					checksum:       artifact.Checksum(),
				}
				return artifacts
			},
		},
		{
			name: "colliding archive name",
			mutate: func(artifacts []VerifiedArtifact) []VerifiedArtifact {
				artifact := artifacts[1]
				artifacts[1] = fakeVerifiedArtifact{
					target:         artifact.Target(),
					sourceRevision: artifact.SourceRevision(),
					archiveName:    artifacts[0].ArchiveName(),
					checksum:       artifact.Checksum(),
				}
				return artifacts
			},
		},
		{
			name: "mismatched checksum",
			mutate: func(artifacts []VerifiedArtifact) []VerifiedArtifact {
				artifact := artifacts[2]
				artifacts[2] = fakeVerifiedArtifact{
					target:         artifact.Target(),
					sourceRevision: artifact.SourceRevision(),
					archiveName:    artifact.ArchiveName(),
					checksum:       "mismatched-checksum",
				}
				return artifacts
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			records := eligibleAggregateRecords(t, sourceSHA)
			workflow := successfulAggregateWorkflow(t, sourceSHA, records)
			workflow.download = writeAggregateDownloadFixtures
			artifacts := test.mutate(verifiedAggregateArtifacts(records))
			output := filepath.Join(t.TempDir(), "dist")

			result, err := RunAggregate(t.Context(), AggregateRequest{
				Ref:             "main",
				OutputDirectory: output,
			}, successfulAggregateDependencies(workflow, &fakeAggregateVerifier{artifacts: artifacts}))
			require.Error(t, err)
			assert.False(t, directoryExists(output), "unverified downloads must not be exposed as success")
			assert.NotEmpty(t, result.QuarantineDirectory)
			assert.DirExists(t, result.QuarantineDirectory)
		})
	}
}

func TestAggregateRunReportsIndependentTargetFailures(t *testing.T) {
	t.Parallel()

	const sourceSHA = "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	windowsFailure := errors.New("Windows native launch failed")
	linuxFailure := errors.New("Linux listener remained active")
	records := eligibleAggregateRecords(t, sourceSHA)
	records[1].Status = AggregateTargetFailed
	records[1].Failure = windowsFailure
	records[3].Status = AggregateTargetFailed
	records[3].Failure = linuxFailure
	workflow := successfulAggregateWorkflow(t, sourceSHA, records)
	reported := make([]AggregateTargetRecord, 0, len(records))
	dependencies := successfulAggregateDependencies(workflow, nil)
	dependencies.Report = func(record AggregateTargetRecord) {
		reported = append(reported, record)
	}

	result, err := RunAggregate(t.Context(), AggregateRequest{
		Ref:             "main",
		OutputDirectory: filepath.Join(t.TempDir(), "dist"),
	}, dependencies)
	require.Error(t, err)
	assert.ErrorIs(t, err, windowsFailure)
	assert.ErrorIs(t, err, linuxFailure)
	assert.Len(t, result.Records, 4)
	assert.Equal(t, aggregateRecordTargets(records), aggregateRecordTargets(reported))
	assert.False(t, workflow.downloadCalled)
}

func TestAggregateRunCancellationAttemptsBoundedRemoteCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	const (
		sourceSHA     = "ffffffffffffffffffffffffffffffffffffffff"
		correlationID = "cancelled-contract-run"
	)
	partialRecord := eligibleAggregateRecords(t, sourceSHA)[0]
	remoteCancelFailure := errors.New("remote cancellation was not confirmed")
	workflow := successfulAggregateWorkflow(t, sourceSHA, nil)
	workflow.found.CorrelationID = correlationID
	workflow.completed.CorrelationID = correlationID
	workflow.wait = func(ctx context.Context, run AggregateRun, report func(AggregateTargetRecord)) (AggregateRun, error) {
		if report != nil {
			report(partialRecord)
		}
		cancel()
		<-ctx.Done()
		run.Records = []AggregateTargetRecord{partialRecord}
		return run, ctx.Err()
	}
	workflow.cancel = func(ctx context.Context, _ AggregateRun) error {
		assert.NoError(t, ctx.Err(), "remote cancellation must not reuse the canceled wait context")
		_, bounded := ctx.Deadline()
		assert.True(t, bounded, "remote cancellation must have a bounded cleanup context")
		return remoteCancelFailure
	}
	dependencies := successfulAggregateDependencies(workflow, nil)
	dependencies.NewCorrelationID = func() string { return correlationID }

	result, err := RunAggregate(ctx, AggregateRequest{
		Ref:             "main",
		OutputDirectory: filepath.Join(t.TempDir(), "dist"),
	}, dependencies)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.ErrorIs(t, err, remoteCancelFailure, "unconfirmed remote cancellation must remain actionable")
	assert.True(t, workflow.cancelCalled)
	assert.False(t, workflow.downloadCalled)
	assert.Equal(t, correlationID, result.CorrelationID)
	assert.Equal(t, []string{partialRecord.Target.String()}, aggregateRecordTargets(result.Records))
}

func TestAggregateRunWaitFailureAttemptsBoundedRemoteCancellation(t *testing.T) {
	t.Parallel()

	const sourceSHA = "abababababababababababababababababababab"
	waitFailure := errors.New("workflow status query failed")
	workflow := successfulAggregateWorkflow(t, sourceSHA, eligibleAggregateRecords(t, sourceSHA))
	workflow.waitErr = waitFailure
	workflow.cancel = func(ctx context.Context, _ AggregateRun) error {
		assert.NoError(t, ctx.Err())
		_, bounded := ctx.Deadline()
		assert.True(t, bounded, "remote cancellation must use a bounded cleanup context")
		return nil
	}

	_, err := RunAggregate(t.Context(), AggregateRequest{
		Ref:             "main",
		OutputDirectory: filepath.Join(t.TempDir(), "dist"),
	}, successfulAggregateDependencies(workflow, nil))
	require.ErrorIs(t, err, waitFailure)
	assert.True(t, workflow.cancelCalled)
	assert.False(t, workflow.downloadCalled)
}

func TestAggregateRunQuarantinesPartialDownloads(t *testing.T) {
	t.Parallel()

	const sourceSHA = "1111111111111111111111111111111111111111"
	downloadFailure := errors.New("artifact download interrupted")
	workflow := successfulAggregateWorkflow(t, sourceSHA, eligibleAggregateRecords(t, sourceSHA))
	workflow.download = func(_ context.Context, destination string) error {
		require.NoError(t, os.MkdirAll(destination, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(destination, "partial.zip"), []byte("partial"), 0o600))
		return downloadFailure
	}
	output := filepath.Join(t.TempDir(), "dist")

	result, err := RunAggregate(t.Context(), AggregateRequest{
		Ref:             "main",
		OutputDirectory: output,
	}, successfulAggregateDependencies(workflow, nil))
	require.Error(t, err)
	assert.ErrorIs(t, err, downloadFailure)
	assert.False(t, directoryExists(output))
	require.NotEmpty(t, result.QuarantineDirectory)
	assert.NotEqual(t, filepath.Clean(output), filepath.Clean(result.QuarantineDirectory))
	assert.FileExists(t, filepath.Join(result.QuarantineDirectory, "partial.zip"))
}

func TestAggregateRunExposesVerifiedDownloadsOnlyAfterAtomicPublish(t *testing.T) {
	t.Parallel()

	const sourceSHA = "2222222222222222222222222222222222222222"
	root := t.TempDir()
	output := filepath.Join(root, "dist")
	records := eligibleAggregateRecords(t, sourceSHA)
	workflow := successfulAggregateWorkflow(t, sourceSHA, records)
	workflow.download = writeAggregateDownloadFixtures
	verifier := &fakeAggregateVerifier{artifacts: verifiedAggregateArtifacts(records)}
	verifier.verify = func(_ context.Context, quarantine string) ([]VerifiedArtifact, error) {
		assert.False(t, directoryExists(output), "success output became visible before verification completed")
		assert.DirExists(t, quarantine)
		assert.FileExists(t, filepath.Join(quarantine, "download-complete"))
		return verifier.artifacts, nil
	}

	result, err := RunAggregate(t.Context(), AggregateRequest{
		Ref:             "main",
		OutputDirectory: output,
	}, successfulAggregateDependencies(workflow, verifier))
	require.NoError(t, err)
	assert.Equal(t, output, result.OutputDirectory)
	assert.DirExists(t, output)
	assert.FileExists(t, filepath.Join(output, "download-complete"))
	assert.False(t, directoryExists(result.QuarantineDirectory), "successful publish must consume its quarantine directory")
}

type fakeAggregateWorkflow struct {
	resolved  AggregateRevision
	found     AggregateRun
	completed AggregateRun

	resolveErr  error
	dispatchErr error
	findErr     error
	waitErr     error

	dispatched     []AggregateDispatch
	discoveryIDs   []string
	waitCalled     bool
	downloadCalled bool
	cancelCalled   bool

	wait     func(context.Context, AggregateRun, func(AggregateTargetRecord)) (AggregateRun, error)
	download func(context.Context, string) error
	cancel   func(context.Context, AggregateRun) error
}

func (workflow *fakeAggregateWorkflow) ResolveRevision(_ context.Context, ref string) (AggregateRevision, error) {
	workflow.resolved.Ref = ref
	return workflow.resolved, workflow.resolveErr
}

func (workflow *fakeAggregateWorkflow) Dispatch(_ context.Context, dispatch AggregateDispatch) error {
	workflow.dispatched = append(workflow.dispatched, dispatch)
	return workflow.dispatchErr
}

func (workflow *fakeAggregateWorkflow) FindRun(_ context.Context, correlationID string) (AggregateRun, error) {
	workflow.discoveryIDs = append(workflow.discoveryIDs, correlationID)
	return workflow.found, workflow.findErr
}

func (workflow *fakeAggregateWorkflow) Wait(
	ctx context.Context,
	run AggregateRun,
	report func(AggregateTargetRecord),
) (AggregateRun, error) {
	workflow.waitCalled = true
	if workflow.wait != nil {
		return workflow.wait(ctx, run, report)
	}
	for _, record := range workflow.completed.Records {
		if report != nil {
			report(record)
		}
	}
	return workflow.completed, workflow.waitErr
}

func (workflow *fakeAggregateWorkflow) Download(ctx context.Context, _ AggregateRun, destination string) error {
	workflow.downloadCalled = true
	if workflow.download == nil {
		return nil
	}
	return workflow.download(ctx, destination)
}

func (workflow *fakeAggregateWorkflow) Cancel(ctx context.Context, run AggregateRun) error {
	workflow.cancelCalled = true
	if workflow.cancel == nil {
		return nil
	}
	return workflow.cancel(ctx, run)
}

type fakeAggregateVerifier struct {
	artifacts []VerifiedArtifact
	err       error
	verify    func(context.Context, string) ([]VerifiedArtifact, error)
}

func (verifier *fakeAggregateVerifier) Verify(ctx context.Context, directory string) ([]VerifiedArtifact, error) {
	if verifier.verify != nil {
		return verifier.verify(ctx, directory)
	}
	return verifier.artifacts, verifier.err
}

type fakeVerifiedArtifact struct {
	target         Target
	sourceRevision string
	archiveName    string
	checksum       string
}

func (artifact fakeVerifiedArtifact) Target() Target         { return artifact.target }
func (artifact fakeVerifiedArtifact) SourceRevision() string { return artifact.sourceRevision }
func (artifact fakeVerifiedArtifact) ArchiveName() string    { return artifact.archiveName }
func (artifact fakeVerifiedArtifact) Checksum() string       { return artifact.checksum }

func successfulAggregateWorkflow(t *testing.T, sourceSHA string, records []AggregateTargetRecord) *fakeAggregateWorkflow {
	t.Helper()

	return &fakeAggregateWorkflow{
		resolved: AggregateRevision{Repository: "vaulttec/fallout-terminal", Ref: "main", SourceSHA: sourceSHA},
		found: AggregateRun{
			ID:            42,
			URL:           "https://example.invalid/actions/runs/42",
			CorrelationID: "aggregate-contract",
			SourceSHA:     sourceSHA,
		},
		completed: AggregateRun{
			ID:            42,
			URL:           "https://example.invalid/actions/runs/42",
			CorrelationID: "aggregate-contract",
			SourceSHA:     sourceSHA,
			Records:       records,
		},
	}
}

func successfulAggregateDependencies(
	workflow *fakeAggregateWorkflow,
	verifier AggregateVerifier,
) AggregateDependencies {
	if verifier == nil {
		verifier = &fakeAggregateVerifier{}
	}
	return AggregateDependencies{
		Workflow:         workflow,
		Verifier:         verifier,
		NewCorrelationID: func() string { return "aggregate-contract" },
	}
}

func eligibleAggregateRecords(t *testing.T, sourceSHA string) []AggregateTargetRecord {
	t.Helper()

	targets := []Target{
		mustParseTarget(t, goosWindows, goarchARM64),
		mustParseTarget(t, goosWindows, goarchAMD64),
		mustParseTarget(t, goosLinux, goarchARM64),
		mustParseTarget(t, goosLinux, goarchAMD64),
	}
	records := make([]AggregateTargetRecord, 0, len(targets))
	for _, target := range targets {
		records = append(records, AggregateTargetRecord{
			Target:      target,
			SourceSHA:   sourceSHA,
			Status:      AggregateTargetEligible,
			ArchiveName: target.ArchiveName(),
			Checksum:    "sha256-" + target.String(),
		})
	}
	return records
}

func verifiedAggregateArtifacts(records []AggregateTargetRecord) []VerifiedArtifact {
	artifacts := make([]VerifiedArtifact, 0, len(records))
	for _, record := range records {
		artifacts = append(artifacts, fakeVerifiedArtifact{
			target:         record.Target,
			sourceRevision: record.SourceSHA,
			archiveName:    record.ArchiveName,
			checksum:       record.Checksum,
		})
	}
	return artifacts
}

func aggregateRecordTargets(records []AggregateTargetRecord) []string {
	targets := make([]string, 0, len(records))
	for _, record := range records {
		targets = append(targets, record.Target.String())
	}
	return targets
}

func writeAggregateDownloadFixtures(_ context.Context, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(destination, "download-complete"), []byte("verified later"), 0o600)
}

func directoryExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

var (
	_ AggregateWorkflow = (*fakeAggregateWorkflow)(nil)
	_ AggregateVerifier = (*fakeAggregateVerifier)(nil)
	_ VerifiedArtifact  = fakeVerifiedArtifact{}
)
