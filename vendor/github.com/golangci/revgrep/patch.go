package revgrep

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type patchOption struct {
	revisionFrom string
	revisionTo   string
	mergeBase    string
}

// GitPatch returns a patch from a git repository.
// If no git repository was found and no errors occurred, nil is returned,
// else an error is returned revisionFrom and revisionTo defines the git diff parameters,
// if left blank and there are unstaged changes or untracked files,
// only those will be returned else only check changes since HEAD~.
// If revisionFrom is set but revisionTo is not,
// untracked files will be included, to exclude untracked files set revisionTo to HEAD~.
// It's incorrect to specify revisionTo without a revisionFrom.
func GitPatch(ctx context.Context, option patchOption) (io.Reader, []string, error) {
	// check if git repo exists
	if err := exec.CommandContext(ctx, "git", "status", "--porcelain").Run(); err != nil {
		// don't return an error, we assume the error is not repo exists
		return nil, nil, nil
	}

	// make a patch for untracked files
	ls, err := exec.CommandContext(ctx, "git", "ls-files", "--others", "--exclude-standard").CombinedOutput()
	if err != nil {
		return nil, nil, fmt.Errorf("error executing git ls-files: %w", err)
	}

	var newFiles []string
	for _, file := range bytes.Split(ls, []byte{'\n'}) {
		if len(file) == 0 || bytes.HasSuffix(file, []byte{'/'}) {
			// ls-files was sometimes showing directories when they were ignored
			// I couldn't create a test case for this as I couldn't reproduce correctly for the moment,
			// just exclude files with trailing /
			continue
		}

		newFiles = append(newFiles, string(file))
	}

	if option.mergeBase != "" {
		var base string
		base, err = getMergeBase(ctx, option.mergeBase)
		if err != nil {
			return nil, nil, err
		}

		if base != "" {
			option.revisionFrom = base
		}
	}

	if option.revisionFrom != "" {
		args := []string{option.revisionFrom}

		if option.revisionTo != "" {
			args = append(args, option.revisionTo)
		}

		args = append(args, "--")

		patch, errDiff := gitDiff(ctx, args...)
		if errDiff != nil {
			return nil, nil, errDiff
		}

		if option.revisionTo == "" {
			return patch, newFiles, nil
		}

		return patch, nil, nil
	}

	// make a patch for unstaged changes
	patch, err := gitDiff(ctx, "--")
	if err != nil {
		return nil, nil, err
	}

	unstaged := patch.Len() > 0

	// If there's unstaged changes OR untracked changes (or both),
	// then this is a suitable patch
	if unstaged || newFiles != nil {
		return patch, newFiles, nil
	}

	// check for changes in recent commit
	patch, err = gitDiff(ctx, "HEAD~", "--")
	if err != nil {
		return nil, nil, err
	}

	return patch, nil, nil
}

func gitDiff(ctx context.Context, extraArgs ...string) (*bytes.Buffer, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--color=never", "--no-ext-diff")

	if isSupportedByGit(ctx, 2, 41, 0) {
		cmd.Args = append(cmd.Args, "--default-prefix")
	}

	cmd.Args = append(cmd.Args, "--relative")
	cmd.Args = append(cmd.Args, extraArgs...)

	patch := new(bytes.Buffer)
	errBuff := new(bytes.Buffer)

	cmd.Stdout = patch
	cmd.Stderr = errBuff

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error executing %q: %w: %w", strings.Join(cmd.Args, " "), err, readAsError(errBuff))
	}

	return patch, nil
}

func readAsError(buff io.Reader) error {
	output, err := io.ReadAll(buff)
	if err != nil {
		return fmt.Errorf("read stderr: %w", err)
	}

	return errors.New(string(output))
}

func isSupportedByGit(ctx context.Context, major, minor, patch int) bool {
	output, err := exec.CommandContext(ctx, "git", "version").CombinedOutput()
	if err != nil {
		return false
	}

	parts := bytes.Split(bytes.TrimSpace(output), []byte(" "))
	if len(parts) < 3 {
		return false
	}

	v := string(parts[2])
	if v == "" {
		return false
	}

	vp := regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?.*$`).FindStringSubmatch(v)
	if len(vp) < 4 {
		return false
	}

	currentMajor, err := strconv.Atoi(vp[1])
	if err != nil {
		return false
	}

	currentMinor, err := strconv.Atoi(vp[2])
	if err != nil {
		return false
	}

	currentPatch, err := strconv.Atoi(vp[3])
	if err != nil {
		return false
	}

	return currentMajor*1_000_000_000+currentMinor*1_000_000+currentPatch*1_000 >= major*1_000_000_000+minor*1_000_000+patch*1_000
}

func getMergeBase(ctx context.Context, base string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "merge-base", base, "HEAD")

	patch := new(bytes.Buffer)
	errBuff := new(bytes.Buffer)

	cmd.Stdout = patch
	cmd.Stderr = errBuff

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error executing %q: %w: %w", strings.Join(cmd.Args, " "), err, readAsError(errBuff))
	}

	return strings.TrimSpace(patch.String()), nil
}
