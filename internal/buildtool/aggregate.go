package buildtool

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	portableWorkflowFile          = "wails-portable.yml"
	portableWorkflowPath          = ".github/workflows/wails-portable.yml"
	portableAggregateArtifactName = "fallout-terminal-portable"
	portableAggregateIndexName    = "aggregate-index.json"
	aggregateCancelTimeout        = 15 * time.Second
	aggregatePollInterval         = 5 * time.Second
)

var (
	aggregateCorrelationPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	aggregateRepositoryPattern  = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	aggregateSHApattern         = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)
)

// AggregateTargetStatus is the terminal eligibility state reported by a
// native package job.
type AggregateTargetStatus string

const (
	AggregateTargetPending  AggregateTargetStatus = "pending"
	AggregateTargetBuilding AggregateTargetStatus = "building"
	AggregateTargetEligible AggregateTargetStatus = "eligible"
	AggregateTargetFailed   AggregateTargetStatus = "failed"
)

// AggregateRevision binds a requested ref to one repository commit.
type AggregateRevision struct {
	Repository string
	Ref        string
	SourceSHA  string
}

// AggregateDispatch is the complete immutable input sent to the portable
// workflow.
type AggregateDispatch struct {
	CorrelationID string
	Revision      AggregateRevision
}

// AggregateTargetRecord is one native target's independent terminal result.
type AggregateTargetRecord struct {
	Target      Target
	SourceSHA   string
	Status      AggregateTargetStatus
	ArchiveName string
	Checksum    string
	Failure     error
}

// AggregateRun identifies the correlated remote workflow and its observed
// target records.
type AggregateRun struct {
	ID            int64
	URL           string
	CorrelationID string
	SourceSHA     string
	Records       []AggregateTargetRecord
}

// AggregateRequest describes one complete portable-matrix request.
type AggregateRequest struct {
	Ref             string
	OutputDirectory string
}

// VerifiedArtifact is the artifact identity consumed by the aggregate join.
type VerifiedArtifact interface {
	Target() Target
	SourceRevision() string
	ArchiveName() string
	Checksum() string
}

// AggregateVerifier validates all downloaded archives while they remain
// quarantined.
type AggregateVerifier interface {
	Verify(context.Context, string) ([]VerifiedArtifact, error)
}

type aggregateRunEvidenceVerifier interface {
	VerifyRunEvidence(context.Context, string, string, string, []VerifiedArtifact) error
}

// AggregateWorkflow owns authenticated remote workflow operations.
type AggregateWorkflow interface {
	ResolveRevision(context.Context, string) (AggregateRevision, error)
	Dispatch(context.Context, AggregateDispatch) error
	FindRun(context.Context, string) (AggregateRun, error)
	Wait(context.Context, AggregateRun, func(AggregateTargetRecord)) (AggregateRun, error)
	Download(context.Context, AggregateRun, string) error
	Cancel(context.Context, AggregateRun) error
}

// AggregateDependencies contains the side-effecting seams required by the
// aggregate coordinator.
type AggregateDependencies struct {
	Workflow         AggregateWorkflow
	Verifier         AggregateVerifier
	NewCorrelationID func() string
	Report           func(AggregateTargetRecord)
}

// AggregateResult retains remote identity and quarantine evidence even when
// an aggregate attempt fails.
type AggregateResult struct {
	CorrelationID       string
	SourceSHA           string
	RunURL              string
	Records             []AggregateTargetRecord
	OutputDirectory     string
	DarwinBundlePath    string
	QuarantineDirectory string
	Artifacts           []VerifiedArtifact
}

// RunAggregate coordinates, validates, and atomically publishes one complete
// four-target workflow result.
func RunAggregate(
	ctx context.Context,
	request AggregateRequest,
	dependencies AggregateDependencies,
) (AggregateResult, error) {
	result := AggregateResult{OutputDirectory: filepath.Clean(request.OutputDirectory)}
	if err := validateAggregateInputs(ctx, request, dependencies); err != nil {
		return result, err
	}
	if err := ensureAggregateOutputAbsent(result.OutputDirectory); err != nil {
		return result, err
	}

	revision, err := dependencies.Workflow.ResolveRevision(ctx, request.Ref)
	if err != nil {
		return result, fmt.Errorf("resolve aggregate source revision: %w", err)
	}
	if err := validateAggregateRevision(revision); err != nil {
		return result, err
	}
	result.SourceSHA = revision.SourceSHA

	correlationID := dependencies.NewCorrelationID()
	if !aggregateCorrelationPattern.MatchString(correlationID) {
		return result, fmt.Errorf("invalid aggregate correlation identifier %q", correlationID)
	}
	result.CorrelationID = correlationID
	dispatch := AggregateDispatch{CorrelationID: correlationID, Revision: revision}
	if err := dependencies.Workflow.Dispatch(ctx, dispatch); err != nil {
		if ctx.Err() != nil {
			return result, cancelDispatchedAggregate(ctx, dependencies.Workflow, correlationID, err)
		}
		return result, fmt.Errorf("dispatch portable workflow: %w", err)
	}

	run, err := dependencies.Workflow.FindRun(ctx, correlationID)
	if err != nil {
		if ctx.Err() != nil {
			return result, cancelDispatchedAggregate(ctx, dependencies.Workflow, correlationID, err)
		}
		return result, fmt.Errorf("find correlated portable workflow run %q: %w", correlationID, err)
	}
	result.RunURL = run.URL
	if err := validateCorrelatedRun(run, correlationID, revision.SourceSHA); err != nil {
		return result, err
	}

	completed, err := dependencies.Workflow.Wait(ctx, run, dependencies.Report)
	result.RunURL = completed.URL
	result.Records = cloneAggregateRecords(completed.Records)
	if err != nil {
		return result, aggregateWaitError(ctx, dependencies.Workflow, completed, err)
	}
	if err := validateCorrelatedRun(completed, correlationID, revision.SourceSHA); err != nil {
		return result, err
	}
	if err := validateAggregateRecords(completed.Records, revision.SourceSHA); err != nil {
		return result, err
	}

	quarantine, err := createAggregateQuarantine(result.OutputDirectory)
	if err != nil {
		return result, err
	}
	result.QuarantineDirectory = quarantine
	if err := dependencies.Workflow.Download(ctx, completed, quarantine); err != nil {
		return result, fmt.Errorf("download workflow run %d into quarantine: %w", completed.ID, err)
	}

	artifacts, err := dependencies.Verifier.Verify(ctx, quarantine)
	if err != nil {
		return result, fmt.Errorf("verify quarantined aggregate download: %w", err)
	}
	if verifier, ok := dependencies.Verifier.(aggregateRunEvidenceVerifier); ok {
		if err := verifier.VerifyRunEvidence(ctx, quarantine, correlationID, revision.SourceSHA, artifacts); err != nil {
			return result, fmt.Errorf("join aggregate index to workflow run: %w", err)
		}
	}
	if err := validateAggregateArtifacts(artifacts, completed.Records, revision.SourceSHA); err != nil {
		return result, err
	}
	result.Artifacts = append([]VerifiedArtifact(nil), artifacts...)
	result.Records = enrichAggregateRecords(result.Records, artifacts)

	if err := ensureAggregateOutputAbsent(result.OutputDirectory); err != nil {
		return result, err
	}
	if err := os.Rename(quarantine, result.OutputDirectory); err != nil {
		return result, fmt.Errorf("atomically publish verified aggregate output: %w", err)
	}
	return result, nil
}

// PackageAll packages the current clean branch through the origin repository's
// native GitHub Actions matrix.
func PackageAll(
	ctx context.Context,
	root string,
	outputDirectory string,
	report func(AggregateTargetRecord),
) (AggregateResult, error) {
	if err := validateRoot(root); err != nil {
		return AggregateResult{}, err
	}
	workflowPath := filepath.Join(root, portableWorkflowPath)
	workflowInfo, err := os.Stat(workflowPath)
	if err != nil {
		return AggregateResult{}, fmt.Errorf("portable workflow %q is unavailable: %w", portableWorkflowPath, err)
	}
	if !workflowInfo.Mode().IsRegular() {
		return AggregateResult{}, fmt.Errorf("portable workflow %q is not a regular file", portableWorkflowPath)
	}
	checkout, err := resolveCurrentCheckout(ctx, root)
	if err != nil {
		return AggregateResult{}, err
	}
	output := outputDirectory
	if !filepath.IsAbs(output) {
		output = filepath.Join(root, output)
	}
	workflow := &githubCLIWorkflow{
		root:         root,
		program:      "gh",
		workflowFile: portableWorkflowFile,
		pollInterval: aggregatePollInterval,
		repository:   checkout.Repository,
		localSHA:     checkout.SourceSHA,
	}
	return RunAggregate(ctx, AggregateRequest{Ref: checkout.Branch, OutputDirectory: output}, AggregateDependencies{
		Workflow:         workflow,
		Verifier:         directoryAggregateVerifier{},
		NewCorrelationID: newAggregateCorrelationID,
		Report:           report,
	})
}

type aggregateCheckout struct {
	Repository string
	Branch     string
	SourceSHA  string
}

func resolveCurrentCheckout(ctx context.Context, root string) (aggregateCheckout, error) {
	status, err := runGitCommand(ctx, root, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return aggregateCheckout{}, fmt.Errorf("inspect current working tree: %w", err)
	}
	if len(bytes.TrimSpace(status)) != 0 {
		return aggregateCheckout{}, errors.New(
			"package:all:remote requires a clean current branch; commit the working-tree changes and push the branch to origin",
		)
	}

	branchOutput, err := runGitCommand(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return aggregateCheckout{}, errors.New("package:all:remote requires a named current branch; detached HEAD is unsupported")
	}
	branch := strings.TrimSpace(string(branchOutput))
	if err := validateGitRef(branch); err != nil {
		return aggregateCheckout{}, fmt.Errorf("invalid current branch: %w", err)
	}

	headOutput, err := runGitCommand(ctx, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return aggregateCheckout{}, fmt.Errorf("resolve current branch revision: %w", err)
	}
	head := strings.ToLower(strings.TrimSpace(string(headOutput)))
	if !aggregateSHApattern.MatchString(head) {
		return aggregateCheckout{}, errors.New("Git returned an invalid current branch revision")
	}

	remoteOutput, err := runGitCommand(ctx, root, "remote", "get-url", "origin")
	if err != nil {
		return aggregateCheckout{}, errors.New("package:all:remote requires a GitHub remote named origin")
	}
	repository, err := githubRepositoryFromRemote(strings.TrimSpace(string(remoteOutput)))
	if err != nil {
		return aggregateCheckout{}, err
	}
	return aggregateCheckout{Repository: repository, Branch: branch, SourceSHA: head}, nil
}

func runGitCommand(ctx context.Context, root string, arguments ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, "git", arguments...)
	command.Dir = root
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, errors.New(detail)
	}
	return stdout.Bytes(), nil
}

func githubRepositoryFromRemote(remote string) (string, error) {
	value := strings.TrimSpace(remote)
	if value == "" {
		return "", errors.New("origin remote URL is empty")
	}

	var repositoryPath string
	switch {
	case strings.HasPrefix(value, "git@github.com:"):
		repositoryPath = strings.TrimPrefix(value, "git@github.com:")
	default:
		parsed, err := url.Parse(value)
		if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
			return "", fmt.Errorf("origin remote %q is not a supported github.com repository", remote)
		}
		repositoryPath = strings.TrimPrefix(parsed.Path, "/")
	}
	repositoryPath = strings.TrimSuffix(strings.TrimSuffix(repositoryPath, "/"), ".git")
	if !aggregateRepositoryPattern.MatchString(repositoryPath) || strings.Count(repositoryPath, "/") != 1 {
		return "", fmt.Errorf("origin remote %q does not identify an owner/repository pair", remote)
	}
	return repositoryPath, nil
}

func validateAggregateInputs(
	ctx context.Context,
	request AggregateRequest,
	dependencies AggregateDependencies,
) error {
	if ctx == nil {
		return errors.New("aggregate context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if request.Ref == "" || strings.TrimSpace(request.Ref) != request.Ref {
		return errors.New("aggregate ref must be non-empty without surrounding whitespace")
	}
	if request.OutputDirectory == "" {
		return errors.New("aggregate output directory must not be empty")
	}
	if nilInterface(dependencies.Workflow) {
		return errors.New("aggregate workflow dependency is required")
	}
	if nilInterface(dependencies.Verifier) {
		return errors.New("aggregate verifier dependency is required")
	}
	if dependencies.NewCorrelationID == nil {
		return errors.New("aggregate correlation identifier source is required")
	}
	return nil
}

func validateAggregateRevision(revision AggregateRevision) error {
	if revision.Repository == "" || revision.Ref == "" || revision.SourceSHA == "" {
		return errors.New("resolved aggregate revision is incomplete")
	}
	return nil
}

func validateCorrelatedRun(run AggregateRun, correlationID, sourceSHA string) error {
	if run.CorrelationID != correlationID {
		return fmt.Errorf(
			"workflow correlation mismatch: expected %q, got %q",
			correlationID,
			run.CorrelationID,
		)
	}
	if run.SourceSHA != sourceSHA {
		return fmt.Errorf("workflow source revision mismatch: expected %q, got %q", sourceSHA, run.SourceSHA)
	}
	if run.ID <= 0 {
		return errors.New("correlated workflow run has an invalid identifier")
	}
	return nil
}

func validateAggregateRecords(records []AggregateTargetRecord, sourceSHA string) error {
	errs := make([]error, 0)
	if len(records) != len(portableMatrixTargets()) {
		errs = append(errs, fmt.Errorf("aggregate requires exactly four target records, got %d", len(records)))
	}
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		target := record.Target.String()
		if !record.Target.Portable() {
			errs = append(errs, fmt.Errorf("aggregate contains unsupported target %q", target))
			continue
		}
		if _, exists := seen[target]; exists {
			errs = append(errs, fmt.Errorf("aggregate contains duplicate target %s", target))
			continue
		}
		seen[target] = struct{}{}
		if record.SourceSHA != sourceSHA {
			errs = append(errs, fmt.Errorf(
				"target %s source revision mismatch: expected %q, got %q",
				target,
				sourceSHA,
				record.SourceSHA,
			))
		}
		if record.ArchiveName != record.Target.ArchiveName() {
			errs = append(errs, fmt.Errorf(
				"target %s archive mismatch: expected %q, got %q",
				target,
				record.Target.ArchiveName(),
				record.ArchiveName,
			))
		}
		if record.Status != AggregateTargetEligible {
			failure := record.Failure
			if failure == nil {
				failure = fmt.Errorf("workflow status is %q", record.Status)
			}
			errs = append(errs, fmt.Errorf("target %s failed: %w", target, failure))
		}
	}
	for _, target := range portableMatrixTargets() {
		if _, exists := seen[target.String()]; !exists {
			errs = append(errs, fmt.Errorf("aggregate is missing target %s", target))
		}
	}
	return errors.Join(errs...)
}

func validateAggregateArtifacts(
	artifacts []VerifiedArtifact,
	records []AggregateTargetRecord,
	sourceSHA string,
) error {
	errs := make([]error, 0)
	if len(artifacts) != len(portableMatrixTargets()) {
		errs = append(errs, fmt.Errorf("aggregate requires exactly four verified artifacts, got %d", len(artifacts)))
	}
	recordsByTarget := make(map[string]AggregateTargetRecord, len(records))
	for _, record := range records {
		recordsByTarget[record.Target.String()] = record
	}
	seenTargets := make(map[string]struct{}, len(artifacts))
	seenNames := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		if nilVerifiedArtifact(artifact) {
			errs = append(errs, errors.New("aggregate verifier returned a nil artifact"))
			continue
		}
		target := artifact.Target()
		targetName := target.String()
		if !target.Portable() {
			errs = append(errs, fmt.Errorf("aggregate verifier returned unsupported target %q", targetName))
			continue
		}
		if _, exists := seenTargets[targetName]; exists {
			errs = append(errs, fmt.Errorf("aggregate verifier returned duplicate target %s", targetName))
			continue
		}
		seenTargets[targetName] = struct{}{}
		if artifact.SourceRevision() != sourceSHA {
			errs = append(errs, fmt.Errorf(
				"verified target %s source revision mismatch: expected %q, got %q",
				targetName,
				sourceSHA,
				artifact.SourceRevision(),
			))
		}
		if artifact.ArchiveName() != target.ArchiveName() {
			errs = append(errs, fmt.Errorf(
				"verified target %s archive mismatch: expected %q, got %q",
				targetName,
				target.ArchiveName(),
				artifact.ArchiveName(),
			))
		}
		if previousTarget, exists := seenNames[artifact.ArchiveName()]; exists {
			errs = append(errs, fmt.Errorf(
				"verified archive name %q collides between %s and %s",
				artifact.ArchiveName(),
				previousTarget,
				targetName,
			))
		} else {
			seenNames[artifact.ArchiveName()] = targetName
		}
		record, exists := recordsByTarget[targetName]
		if !exists {
			errs = append(errs, fmt.Errorf("verified target %s has no workflow record", targetName))
			continue
		}
		if record.ArchiveName != artifact.ArchiveName() {
			errs = append(errs, fmt.Errorf(
				"verified target %s archive %q does not match workflow record %q",
				targetName,
				artifact.ArchiveName(),
				record.ArchiveName,
			))
		}
		if record.Checksum != "" && record.Checksum != artifact.Checksum() {
			errs = append(errs, fmt.Errorf("verified target %s checksum does not match its workflow record", targetName))
		}
		if artifact.Checksum() == "" {
			errs = append(errs, fmt.Errorf("verified target %s has an empty checksum", targetName))
		}
	}
	for _, target := range portableMatrixTargets() {
		if _, exists := seenTargets[target.String()]; !exists {
			errs = append(errs, fmt.Errorf("aggregate verification is missing target %s", target))
		}
	}
	return errors.Join(errs...)
}

func aggregateWaitError(
	ctx context.Context,
	workflow AggregateWorkflow,
	run AggregateRun,
	waitErr error,
) error {
	interruption := ctx.Err()
	if interruption == nil {
		if errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded) {
			interruption = waitErr
		} else {
			interruption = fmt.Errorf("wait for portable workflow run %d: %w", run.ID, waitErr)
		}
	}
	cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), aggregateCancelTimeout)
	defer cancel()
	cancelErr := workflow.Cancel(cancelCtx, run)
	if cancelErr != nil {
		return errors.Join(interruption, fmt.Errorf("remote workflow cancellation was not confirmed: %w", cancelErr))
	}
	return interruption
}

func cancelDispatchedAggregate(
	ctx context.Context,
	workflow AggregateWorkflow,
	correlationID string,
	operationErr error,
) error {
	interruption := ctx.Err()
	if interruption == nil {
		interruption = operationErr
	}
	cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), aggregateCancelTimeout)
	defer cancel()
	run, findErr := workflow.FindRun(cancelCtx, correlationID)
	if findErr != nil {
		return errors.Join(interruption, fmt.Errorf("find dispatched workflow for remote cancellation: %w", findErr))
	}
	if cancelErr := workflow.Cancel(cancelCtx, run); cancelErr != nil {
		return errors.Join(interruption, fmt.Errorf("remote workflow cancellation was not confirmed: %w", cancelErr))
	}
	return interruption
}

func createAggregateQuarantine(outputDirectory string) (string, error) {
	parent := filepath.Dir(outputDirectory)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fmt.Errorf("create aggregate output parent: %w", err)
	}
	quarantine, err := os.MkdirTemp(parent, "."+filepath.Base(outputDirectory)+".quarantine-")
	if err != nil {
		return "", fmt.Errorf("create aggregate download quarantine: %w", err)
	}
	return quarantine, nil
}

func ensureAggregateOutputAbsent(outputDirectory string) error {
	_, err := os.Lstat(outputDirectory)
	switch {
	case err == nil:
		return fmt.Errorf("aggregate output already exists: %q", outputDirectory)
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("inspect aggregate output %q: %w", outputDirectory, err)
	}
}

func cloneAggregateRecords(records []AggregateTargetRecord) []AggregateTargetRecord {
	return append([]AggregateTargetRecord(nil), records...)
}

func enrichAggregateRecords(
	records []AggregateTargetRecord,
	artifacts []VerifiedArtifact,
) []AggregateTargetRecord {
	artifactsByTarget := make(map[string]VerifiedArtifact, len(artifacts))
	for _, artifact := range artifacts {
		artifactsByTarget[artifact.Target().String()] = artifact
	}
	enriched := cloneAggregateRecords(records)
	for index := range enriched {
		artifact := artifactsByTarget[enriched[index].Target.String()]
		enriched[index].ArchiveName = artifact.ArchiveName()
		enriched[index].Checksum = artifact.Checksum()
	}
	return enriched
}

func nilVerifiedArtifact(artifact VerifiedArtifact) bool {
	return nilInterface(artifact)
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func portableMatrixTargets() []Target {
	return []Target{
		{goos: goosWindows, goarch: goarchARM64},
		{goos: goosWindows, goarch: goarchAMD64},
		{goos: goosLinux, goarch: goarchARM64},
		{goos: goosLinux, goarch: goarchAMD64},
	}
}

func newAggregateCorrelationID() string {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return "package-all-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 36)
	}
	return "package-all-" + hex.EncodeToString(random)
}

type githubCLIWorkflow struct {
	root         string
	program      string
	workflowFile string
	pollInterval time.Duration
	repository   string
	localSHA     string
}

func (workflow *githubCLIWorkflow) ResolveRevision(ctx context.Context, ref string) (AggregateRevision, error) {
	if err := validateGitRef(ref); err != nil {
		return AggregateRevision{}, err
	}
	if !aggregateRepositoryPattern.MatchString(workflow.repository) {
		return AggregateRevision{}, errors.New("origin resolved to an invalid GitHub repository identity")
	}
	if !aggregateSHApattern.MatchString(workflow.localSHA) {
		return AggregateRevision{}, errors.New("local checkout has an invalid source revision")
	}

	endpoint := "repos/" + workflow.repository + "/commits/" + escapeGitHubPathSegment(ref)
	output, err := workflow.run(ctx, "api", endpoint)
	if err != nil {
		return AggregateRevision{}, fmt.Errorf(
			"resolve current branch %q in origin repository %s; push the branch before package:all:remote: %w",
			ref,
			workflow.repository,
			err,
		)
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(output, &commit); err != nil {
		return AggregateRevision{}, fmt.Errorf("decode resolved Git commit: %w", err)
	}
	if !aggregateSHApattern.MatchString(commit.SHA) {
		return AggregateRevision{}, errors.New("GitHub returned an invalid full source revision")
	}
	remoteSHA := strings.ToLower(commit.SHA)
	if remoteSHA != workflow.localSHA {
		return AggregateRevision{}, fmt.Errorf(
			"current branch %q is not pushed to origin %s: local HEAD is %s, remote is %s",
			ref,
			workflow.repository,
			workflow.localSHA,
			remoteSHA,
		)
	}
	return AggregateRevision{Repository: workflow.repository, Ref: ref, SourceSHA: remoteSHA}, nil
}

func (workflow *githubCLIWorkflow) Dispatch(ctx context.Context, dispatch AggregateDispatch) error {
	if !aggregateCorrelationPattern.MatchString(dispatch.CorrelationID) {
		return errors.New("refusing to dispatch an invalid aggregate correlation identifier")
	}
	if !aggregateSHApattern.MatchString(dispatch.Revision.SourceSHA) {
		return errors.New("refusing to dispatch an invalid aggregate source revision")
	}
	if dispatch.Revision.Repository != workflow.repository {
		return errors.New("refusing to dispatch a repository other than the current origin")
	}
	if _, err := workflow.run(
		ctx,
		"workflow", "view", workflow.workflowFile,
		"--repo", dispatch.Revision.Repository,
		"--yaml",
	); err != nil {
		return fmt.Errorf(
			"portable workflow %q is not installed on the default branch of %s; merge the workflow once before using package:all:remote from feature branches: %w",
			workflow.workflowFile,
			dispatch.Revision.Repository,
			err,
		)
	}
	_, err := workflow.run(
		ctx,
		"workflow", "run", workflow.workflowFile,
		"--repo", dispatch.Revision.Repository,
		"--ref", dispatch.Revision.Ref,
		"--field", "correlation_id="+dispatch.CorrelationID,
		"--field", "source_sha="+dispatch.Revision.SourceSHA,
	)
	return err
}

func (workflow *githubCLIWorkflow) FindRun(ctx context.Context, correlationID string) (AggregateRun, error) {
	for {
		runs, err := workflow.listRuns(ctx)
		if err != nil {
			return AggregateRun{}, err
		}
		matching := make([]githubRunDocument, 0, 1)
		for _, run := range runs {
			if containsCorrelation(run.DisplayTitle, correlationID) {
				matching = append(matching, run)
			}
		}
		switch len(matching) {
		case 0:
			if err := waitAggregatePoll(ctx, workflow.pollInterval); err != nil {
				return AggregateRun{}, err
			}
		case 1:
			return matching[0].aggregateRun(correlationID), nil
		default:
			return AggregateRun{}, fmt.Errorf("multiple workflow runs carry correlation identifier %q", correlationID)
		}
	}
}

func (workflow *githubCLIWorkflow) Wait(
	ctx context.Context,
	run AggregateRun,
	report func(AggregateTargetRecord),
) (AggregateRun, error) {
	current := run
	for {
		document, err := workflow.viewRun(ctx, run.ID)
		if err != nil {
			return current, err
		}
		current = document.aggregateRun(run.CorrelationID)
		current.Records = document.targetRecords()
		if document.Status == "completed" {
			for _, record := range current.Records {
				if report != nil {
					report(record)
				}
			}
			return current, nil
		}
		if err := waitAggregatePoll(ctx, workflow.pollInterval); err != nil {
			return current, err
		}
	}
}

func (workflow *githubCLIWorkflow) Download(
	ctx context.Context,
	run AggregateRun,
	destination string,
) error {
	_, err := workflow.run(
		ctx,
		"run", "download", strconv.FormatInt(run.ID, 10),
		"--repo", workflow.repository,
		"--name", portableAggregateArtifactName,
		"--dir", destination,
	)
	return err
}

func (workflow *githubCLIWorkflow) Cancel(ctx context.Context, run AggregateRun) error {
	_, err := workflow.run(
		ctx,
		"run", "cancel", strconv.FormatInt(run.ID, 10),
		"--repo", workflow.repository,
	)
	return err
}

func (workflow *githubCLIWorkflow) listRuns(ctx context.Context) ([]githubRunDocument, error) {
	output, err := workflow.run(
		ctx,
		"run", "list",
		"--repo", workflow.repository,
		"--workflow", workflow.workflowFile,
		"--event", "workflow_dispatch",
		"--limit", "50",
		"--json", "databaseId,url,headSha,displayTitle,status,conclusion",
	)
	if err != nil {
		return nil, err
	}
	var runs []githubRunDocument
	if err := json.Unmarshal(output, &runs); err != nil {
		return nil, fmt.Errorf("decode GitHub workflow run list: %w", err)
	}
	return runs, nil
}

func (workflow *githubCLIWorkflow) viewRun(ctx context.Context, runID int64) (githubRunDocument, error) {
	output, err := workflow.run(
		ctx,
		"run", "view", strconv.FormatInt(runID, 10),
		"--repo", workflow.repository,
		"--json", "databaseId,url,headSha,displayTitle,status,conclusion,jobs",
	)
	if err != nil {
		return githubRunDocument{}, err
	}
	var run githubRunDocument
	if err := decodeSingleJSON(output, &run); err != nil {
		return githubRunDocument{}, fmt.Errorf("decode GitHub workflow run: %w", err)
	}
	return run, nil
}

func (workflow *githubCLIWorkflow) run(ctx context.Context, arguments ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, workflow.program, arguments...)
	command.Dir = workflow.root
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("GitHub CLI is required; install gh and authenticate with `gh auth login`")
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = "no diagnostic output"
		}
		return nil, fmt.Errorf("GitHub CLI command failed (%v): %s", err, detail)
	}
	return stdout.Bytes(), nil
}

type githubRunDocument struct {
	ID           int64               `json:"databaseId"`
	URL          string              `json:"url"`
	HeadSHA      string              `json:"headSha"`
	DisplayTitle string              `json:"displayTitle"`
	Status       string              `json:"status"`
	Conclusion   string              `json:"conclusion"`
	Jobs         []githubJobDocument `json:"jobs"`
}

type githubJobDocument struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

func (document githubRunDocument) aggregateRun(correlationID string) AggregateRun {
	return AggregateRun{
		ID:            document.ID,
		URL:           document.URL,
		CorrelationID: correlationID,
		SourceSHA:     document.HeadSHA,
	}
}

func (document githubRunDocument) targetRecords() []AggregateTargetRecord {
	records := make([]AggregateTargetRecord, 0, len(portableMatrixTargets()))
	for _, job := range document.Jobs {
		target, ok := targetFromJobName(job.Name)
		if !ok {
			continue
		}
		record := AggregateTargetRecord{
			Target:      target,
			SourceSHA:   document.HeadSHA,
			Status:      githubTargetStatus(job.Status, job.Conclusion),
			ArchiveName: target.ArchiveName(),
		}
		if record.Status == AggregateTargetFailed {
			record.Failure = fmt.Errorf("workflow job concluded %q", job.Conclusion)
		}
		records = append(records, record)
	}
	return records
}

func githubTargetStatus(status, conclusion string) AggregateTargetStatus {
	if status != "completed" {
		if status == "in_progress" {
			return AggregateTargetBuilding
		}
		return AggregateTargetPending
	}
	if conclusion == "success" {
		return AggregateTargetEligible
	}
	return AggregateTargetFailed
}

func targetFromJobName(name string) (Target, bool) {
	var matched Target
	matches := 0
	for _, target := range portableMatrixTargets() {
		if containsIdentifier(name, target.String()) {
			matched = target
			matches++
		}
	}
	return matched, matches == 1
}

func containsCorrelation(title, correlationID string) bool {
	return containsIdentifier(title, correlationID)
}

func containsIdentifier(value, identifier string) bool {
	for offset := 0; ; {
		index := strings.Index(value[offset:], identifier)
		if index < 0 {
			return false
		}
		index += offset
		beforeOK := index == 0 || !aggregateIdentifierByte(value[index-1])
		after := index + len(identifier)
		afterOK := after == len(value) || !aggregateIdentifierByte(value[after])
		if beforeOK && afterOK {
			return true
		}
		offset = index + 1
	}
}

func aggregateIdentifierByte(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		value == '.' || value == '_' || value == '-'
}

func waitAggregatePoll(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateGitRef(ref string) error {
	if ref == "" || strings.TrimSpace(ref) != ref || strings.HasPrefix(ref, "-") {
		return errors.New("Git ref must be non-empty and must not start with a flag prefix")
	}
	for _, character := range ref {
		if character < 0x20 || character == 0x7f {
			return errors.New("Git ref contains control characters")
		}
	}
	if strings.ContainsAny(ref, `~^:?*[\`) ||
		strings.Contains(ref, "..") ||
		strings.Contains(ref, "@{") ||
		strings.Contains(ref, "//") ||
		strings.HasSuffix(ref, ".") ||
		strings.HasSuffix(ref, "/") {
		return errors.New("Git ref contains unsupported characters")
	}
	return nil
}

func escapeGitHubPathSegment(value string) string {
	return url.PathEscape(value)
}

func decodeSingleJSON(contents []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing JSON content")
	}
	return nil
}

type directoryAggregateVerifier struct{}

func (directoryAggregateVerifier) Verify(ctx context.Context, directory string) ([]VerifiedArtifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read aggregate download directory: %w", err)
	}
	expected := make(map[string]struct{}, len(portableMatrixTargets())*2)
	for _, target := range portableMatrixTargets() {
		expected[target.ArchiveName()] = struct{}{}
		expected[target.ArchiveName()+".sha256"] = struct{}{}
	}
	indexPresent := false
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect aggregate download entry %q: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("aggregate download entry %q is not a regular file", entry.Name())
		}
		if entry.Name() == portableAggregateIndexName {
			indexPresent = true
			continue
		}
		if _, ok := expected[entry.Name()]; !ok {
			return nil, fmt.Errorf("aggregate download contains unexpected file %q", entry.Name())
		}
	}
	if !indexPresent {
		return nil, fmt.Errorf("aggregate download is missing %s", portableAggregateIndexName)
	}
	artifacts := make([]VerifiedArtifact, 0, len(portableMatrixTargets()))
	for _, target := range portableMatrixTargets() {
		archivePath := filepath.Join(directory, target.ArchiveName())
		checksumPath := archivePath + ".sha256"
		verified, err := VerifyArtifact(ctx, archivePath, checksumPath, target)
		if err != nil {
			return nil, fmt.Errorf("verify %s aggregate artifact: %w", target, err)
		}
		artifacts = append(artifacts, verified)
	}
	if err := verifyAggregateIndex(ctx, filepath.Join(directory, portableAggregateIndexName), "", "", artifacts); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (directoryAggregateVerifier) VerifyRunEvidence(
	ctx context.Context,
	directory string,
	correlationID string,
	sourceSHA string,
	artifacts []VerifiedArtifact,
) error {
	return verifyAggregateIndex(
		ctx,
		filepath.Join(directory, portableAggregateIndexName),
		correlationID,
		sourceSHA,
		artifacts,
	)
}

type aggregateIndexDocument struct {
	SchemaVersion  int                      `json:"schemaVersion"`
	CorrelationID  string                   `json:"correlationID"`
	SourceRevision string                   `json:"sourceRevision"`
	Artifacts      []aggregateIndexArtifact `json:"artifacts"`
}

type aggregateIndexArtifact struct {
	Target      string `json:"target"`
	ArchiveName string `json:"archiveName"`
	Checksum    string `json:"checksum"`
}

func verifyAggregateIndex(
	ctx context.Context,
	indexPath string,
	expectedCorrelationID string,
	expectedSourceSHA string,
	artifacts []VerifiedArtifact,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := requireRegularVerificationFile("aggregate index", indexPath, maxManifestSize); err != nil {
		return err
	}
	contents, err := readLimitedFile(ctx, indexPath, maxManifestSize)
	if err != nil {
		return fmt.Errorf("read aggregate index: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var index aggregateIndexDocument
	if err := decoder.Decode(&index); err != nil {
		return fmt.Errorf("decode aggregate index: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("aggregate index contains unexpected trailing JSON content")
	}
	if index.SchemaVersion != 1 {
		return fmt.Errorf("unsupported aggregate index schema version %d", index.SchemaVersion)
	}
	if !aggregateCorrelationPattern.MatchString(index.CorrelationID) {
		return errors.New("aggregate index has an invalid correlation identifier")
	}
	if expectedCorrelationID != "" && index.CorrelationID != expectedCorrelationID {
		return fmt.Errorf(
			"aggregate index correlation mismatch: expected %q, got %q",
			expectedCorrelationID,
			index.CorrelationID,
		)
	}
	if !aggregateSHApattern.MatchString(index.SourceRevision) {
		return errors.New("aggregate index has an invalid source revision")
	}
	if expectedSourceSHA != "" && index.SourceRevision != expectedSourceSHA {
		return fmt.Errorf(
			"aggregate index source revision mismatch: expected %q, got %q",
			expectedSourceSHA,
			index.SourceRevision,
		)
	}
	if len(index.Artifacts) != len(portableMatrixTargets()) {
		return fmt.Errorf("aggregate index requires exactly four artifacts, got %d", len(index.Artifacts))
	}

	verified := make(map[string]VerifiedArtifact, len(artifacts))
	for _, artifact := range artifacts {
		verified[artifact.Target().String()] = artifact
	}
	seen := make(map[string]struct{}, len(index.Artifacts))
	for _, indexed := range index.Artifacts {
		if _, exists := seen[indexed.Target]; exists {
			return fmt.Errorf("aggregate index contains duplicate target %q", indexed.Target)
		}
		seen[indexed.Target] = struct{}{}
		artifact, exists := verified[indexed.Target]
		if !exists {
			return fmt.Errorf("aggregate index contains unexpected target %q", indexed.Target)
		}
		if artifact.SourceRevision() != index.SourceRevision {
			return fmt.Errorf("aggregate index source revision does not match target %s", indexed.Target)
		}
		if indexed.ArchiveName != artifact.ArchiveName() {
			return fmt.Errorf("aggregate index archive does not match target %s", indexed.Target)
		}
		if indexed.Checksum != artifact.Checksum() {
			return fmt.Errorf("aggregate index checksum does not match target %s", indexed.Target)
		}
	}
	return nil
}

var (
	_ AggregateWorkflow = (*githubCLIWorkflow)(nil)
	_ AggregateVerifier = directoryAggregateVerifier{}
)
