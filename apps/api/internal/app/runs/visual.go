package runs

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/gjovanovicst/ai-engineering-platform/internal/app/visualdiff"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// visualScreenshots extracts PNG files under visual/ from the run workspace.
func visualScreenshots(archive []byte) map[string][]byte {
	out := map[string][]byte{}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			return out
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		if hdr.Typeflag != tar.TypeReg || !strings.HasPrefix(name, "visual/") || !strings.HasSuffix(strings.ToLower(name), ".png") || hdr.Size > 8<<20 {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(tr, 8<<20))
		out[strings.TrimSuffix(path.Base(name), ".png")] = data
	}
}

// checkVisualBaselines compares each captured screenshot with its stored
// baseline. Missing baselines are recorded, not failures (first run).
func (r *Runner) checkVisualBaselines(ctx context.Context, run domain.Run, archive []byte, threshold float64, masks []visualdiff.Rect) (string, error) {
	screens := visualScreenshots(archive)
	if len(screens) == 0 {
		return "", nil
	}
	var failures []string
	for name, actual := range screens {
		if r.artifacts != nil {
			_ = r.artifacts.Upload(ctx, run.OrgID, run.ID, "visual/"+name, actual)
		}
		key := "visual/" + run.ProjectID + "/" + name + ".png"
		baseline, err := r.visuals.Get(ctx, key)
		if err != nil || len(baseline) == 0 {
			_ = r.store.AppendEvent(ctx, run.OrgID, run.ID, "visual.baseline_missing", "system", map[string]any{"name": name})
			continue
		}
		result, derr := visualdiff.ComparePNG(bytes.NewReader(baseline), bytes.NewReader(actual), threshold, masks)
		if derr != nil {
			failures = append(failures, name+": "+derr.Error())
			continue
		}
		_ = r.store.AppendEvent(ctx, run.OrgID, run.ID, "visual.baseline_diff", "system",
			map[string]any{"name": name, "diff_percent": result.DiffPercent, "passed": result.Passed})
		if !result.Passed {
			failures = append(failures, fmt.Sprintf("%s: %.2f%% changed (limit %.2f%%)", name, result.DiffPercent, result.Threshold))
		}
	}
	if len(failures) > 0 {
		return "visual baseline mismatch: " + strings.Join(failures, "; "), fmt.Errorf("visual baseline mismatch")
	}
	return "", nil
}
