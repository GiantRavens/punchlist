package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"punchlist/config"
	"punchlist/task"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// renumber repairs duplicate task ids (pin #49).
//
// A store shared between hosts can still collide when sync is stalled: each
// host allocates above the highest id IT can see (1.3.5's disk floor), and
// neither can see the other's file until sync resumes. Observed 2026-10-23 in
// the notebook root: futhark's 271 (an open ticket) and the Mac's 271 (a DONE
// log entry created while Mac Syncthing was down). Every by-id command on 271
// then refuses as ambiguous, so the open ticket was unreachable. `pin doctor`
// (duplicate_id) and the fleet pin_scope_integrity sensor already DETECT this;
// renumber is the repair.

type renumberMove struct {
	oldPath string
	newPath string
	oldID   int
	newID   int
	reason  string
}

func newRenumberCmd() *cobra.Command {
	var collisions, dryRun bool
	cmd := &cobra.Command{
		Use:   "renumber [task-file]",
		Short: "Give a task file a fresh id (repairs duplicate ids from synced stores)",
		Long: "Move one task file to the next free id, or with --collisions repair every duplicate id " +
			"in the scope: in each group the earliest-created task keeps the id and the others move to " +
			"fresh ids. The task's frontmatter and filename are rewritten together and its ## Log records " +
			"old -> new, so references elsewhere can be fixed by hand. A task file is named by path, or by " +
			"bare filename inside this scope's tasks/ (an ambiguous id cannot name one). --dry-run prints " +
			"the plan and changes nothing.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if collisions == (len(args) == 1) {
				return fmt.Errorf("give exactly one of: a task file, or --collisions")
			}
			root, err := config.FindPunchlistRoot()
			if err != nil {
				if printNotPunchlistError(err) {
					return nil
				}
				return err
			}
			return withProjectWriteLock(root, func() error {
				moves, err := planRenumber(args, collisions)
				if err != nil {
					return err
				}
				if len(moves) == 0 {
					fmt.Println("No duplicate ids.")
					return nil
				}
				for _, m := range moves {
					verb := "Renumbered"
					if dryRun {
						verb = "Would renumber"
					}
					fmt.Printf("%s %d -> %d: %s (%s)\n", verb, m.oldID, m.newID, filepath.Base(m.oldPath), m.reason)
					if dryRun {
						continue
					}
					if err := applyRenumber(m); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&collisions, "collisions", false, "repair every duplicate id (the earliest-created task keeps it)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan, change nothing")
	return cmd
}

// planRenumber decides every move before any file changes, allocating ids in order above both the
// cached next_id and the highest id on disk (tasks/ and .trash/), the same floor creation uses.
func planRenumber(args []string, collisions bool) ([]renumberMove, error) {
	tasksPath, err := tasksDir()
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	idWidth := idWidthFromConfig(cfg)
	next := cfg.NextID
	if floor := maxOnDiskID(tasksPath) + 1; floor > next {
		next = floor
	}

	type victim struct {
		path   string
		reason string
	}
	var victims []victim
	if collisions {
		groups, err := duplicateIDGroups(tasksPath)
		if err != nil {
			return nil, err
		}
		for _, g := range groups {
			keeper := filepath.Base(g[0].Path)
			for _, t := range g[1:] {
				victims = append(victims, victim{t.Path, "id collision; " + keeper + " keeps it"})
			}
		}
	} else {
		p, err := resolveTaskFileArg(args[0], tasksPath)
		if err != nil {
			return nil, err
		}
		victims = append(victims, victim{p, "renumbered by request"})
	}

	moves := make([]renumberMove, 0, len(victims))
	for _, v := range victims {
		t, err := task.Parse(v.path)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %s: %w", v.path, err)
		}
		newPath := compactTargetPath(filepath.Dir(v.path), t, compactSuffix(filepath.Base(v.path), t.Title), next, idWidth)
		if _, err := os.Stat(newPath); err == nil {
			return nil, fmt.Errorf("refusing to overwrite %s", newPath)
		}
		moves = append(moves, renumberMove{oldPath: v.path, newPath: newPath, oldID: t.ID, newID: next, reason: v.reason})
		next++
	}
	return moves, nil
}

// applyRenumber renames first, then rewrites the frontmatter in place. A crash between the two
// leaves a filename/frontmatter mismatch that `pin doctor` reports as id_mismatch, never a silent
// second copy.
func applyRenumber(m renumberMove) error {
	t, err := task.Parse(m.oldPath)
	if err != nil {
		return err
	}
	if err := os.Rename(m.oldPath, m.newPath); err != nil {
		return fmt.Errorf("failed to move %s: %w", m.oldPath, err)
	}
	t.ID = m.newID
	t.Path = m.newPath
	addLog(t, fmt.Sprintf("renumbered id from %d to %d (%s)", m.oldID, m.newID, m.reason))
	if err := t.Write(m.newPath); err != nil {
		return fmt.Errorf("moved to %s but failed to rewrite its id: %w", m.newPath, err)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if cfg.NextID <= m.newID {
		cfg.NextID = m.newID + 1
		if err := config.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	return nil
}

// duplicateIDGroups returns each set of task files sharing a filename id (the key findTaskFile
// resolves by), earliest-created first; ties break on path so the plan is deterministic.
func duplicateIDGroups(tasksPath string) ([][]*task.Task, error) {
	byID := map[int][]*task.Task{}
	err := filepath.WalkDir(tasksPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		id := leadingTaskID(d.Name())
		if id == 0 {
			return nil
		}
		t, perr := task.Parse(path)
		if perr != nil {
			return fmt.Errorf("cannot parse %s: %w", path, perr)
		}
		byID[id] = append(byID[id], t)
		return nil
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(byID))
	for id, g := range byID {
		if len(g) > 1 {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	groups := make([][]*task.Task, 0, len(ids))
	for _, id := range ids {
		g := byID[id]
		sort.Slice(g, func(i, j int) bool {
			if !g[i].CreatedAt.Equal(g[j].CreatedAt) {
				return g[i].CreatedAt.Before(g[j].CreatedAt)
			}
			return g[i].Path < g[j].Path
		})
		groups = append(groups, g)
	}
	return groups, nil
}

// resolveTaskFileArg accepts a path, or a bare filename inside tasks/, and refuses anything
// outside this scope's tasks directory.
func resolveTaskFileArg(arg string, tasksPath string) (string, error) {
	cand := arg
	if !filepath.IsAbs(cand) && !strings.ContainsRune(cand, os.PathSeparator) {
		cand = filepath.Join(tasksPath, cand)
	}
	abs, err := filepath.Abs(cand)
	if err != nil {
		return "", err
	}
	absTasks, err := filepath.Abs(tasksPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absTasks, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%s is not a task file in %s", arg, tasksPath)
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() || !strings.HasSuffix(abs, ".md") {
		return "", fmt.Errorf("%s is not a task file", arg)
	}
	return abs, nil
}
