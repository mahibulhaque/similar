package algorithms

import (
	"github.com/mahibulhaque/similar/internal/diff"
	"github.com/mahibulhaque/similar/internal/diffutil"
)

// maybeEmitSmallSideExact runs an exact O(N*M) LCS fallback when one side is
// tiny and the other large, staying minimal and fast in that shape. It reports
// whether it emitted the script.
func maybeEmitSmallSideExact[T comparable](
	d diff.DiffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	dl deadline,
) (bool, error) {
	if dl.exceeded() {
		return false, nil
	}

	oldLen := oldEnd - oldStart
	newLen := newEnd - newStart
	small := min(oldLen, newLen)
	large := max(oldLen, newLen)
	work := diffutil.SatMul(oldLen, newLen)

	if small == 0 ||
		small > smallSideExactMax ||
		large < smallSideExactMinLarge ||
		work > smallSideExactMaxWork {
		return false, nil
	}

	if oldLen <= newLen {
		return emitSmallOldExact(d, old, oldStart, oldEnd, new, newStart, newEnd, dl)
	}
	return emitSmallNewExact(d, old, oldStart, oldEnd, new, newStart, newEnd, dl)
}

// emitSmallOldExact handles the tiny-old / large-new shape. dp[i*width+j] is
// the LCS length of old[i:] vs new[j:]; values are bounded by the small-side
// cap (64), so uint8 suffices.
func emitSmallOldExact[T comparable](
	d diff.DiffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	dl deadline,
) (bool, error) {
	if dl.exceeded() {
		return false, nil
	}

	n := oldEnd - oldStart
	m := newEnd - newStart
	width := m + 1

	dp := make([]uint8, (n+1)*width)
	for i := n - 1; i >= 0; i-- {
		if dl.exceeded() {
			return false, nil
		}
		row := i * width
		nextRow := (i + 1) * width
		for j := m - 1; j >= 0; j-- {
			if j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
				return false, nil
			}
			if new[newStart+j] == old[oldStart+i] {
				dp[row+j] = dp[nextRow+j+1] + 1
			} else {
				dp[row+j] = max(dp[nextRow+j], dp[row+j+1])
			}
		}
	}

	emittedAny := false
	i, j := 0, 0
	for i < n && j < m {
		if !emittedAny && j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
			return false, nil
		}
		row := i * width
		oldIdx := oldStart + i
		newIdx := newStart + j

		if new[newIdx] == old[oldIdx] && dp[row+j] == dp[(i+1)*width+j+1]+1 {
			startI, startJ := i, j
			for i < n && j < m {
				if !emittedAny && j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
					return false, nil
				}
				row := i * width
				if new[newStart+j] == old[oldStart+i] && dp[row+j] == dp[(i+1)*width+j+1]+1 {
					i++
					j++
				} else {
					break
				}
			}
			if err := d.Equal(oldStart+startI, newStart+startJ, i-startI); err != nil {
				return false, err
			}
			emittedAny = true
		} else if dp[(i+1)*width+j] >= dp[row+j+1] {
			startI := i
			delNewIdx := newStart + j
			for i < n {
				if !emittedAny && j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
					return false, nil
				}
				if j >= m {
					i = n
					break
				}
				row := i * width
				if new[newStart+j] == old[oldStart+i] && dp[row+j] == dp[(i+1)*width+j+1]+1 {
					break
				}
				if dp[(i+1)*width+j] >= dp[row+j+1] {
					i++
				} else {
					break
				}
			}
			if err := d.Delete(oldStart+startI, i-startI, delNewIdx); err != nil {
				return false, err
			}
			emittedAny = true
		} else {
			startJ := j
			insOldIdx := oldStart + i
			for j < m {
				if !emittedAny && j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
					return false, nil
				}
				if i >= n {
					j = m
					break
				}
				row := i * width
				if new[newStart+j] == old[oldStart+i] && dp[row+j] == dp[(i+1)*width+j+1]+1 {
					break
				}
				if dp[(i+1)*width+j] < dp[row+j+1] {
					j++
				} else {
					break
				}
			}
			if err := d.Insert(insOldIdx, newStart+startJ, j-startJ); err != nil {
				return false, err
			}
			emittedAny = true
		}
	}

	if i < n {
		if err := d.Delete(oldStart+i, n-i, newStart+j); err != nil {
			return false, err
		}
	}
	if j < m {
		if err := d.Insert(oldStart+i, newStart+j, m-j); err != nil {
			return false, err
		}
	}

	return true, nil
}

// emitSmallNewExact is the mirror of emitSmallOldExact for the tiny-new /
// large-old shape. dp[i*width+j] is the LCS length of new[i:] vs old[j:].
func emitSmallNewExact[T comparable](
	d diff.DiffHook,
	old []T, oldStart, oldEnd int,
	new []T, newStart, newEnd int,
	dl deadline,
) (bool, error) {
	if dl.exceeded() {
		return false, nil
	}

	n := oldEnd - oldStart
	m := newEnd - newStart
	width := n + 1

	dp := make([]uint8, (m+1)*width)
	for i := m - 1; i >= 0; i-- {
		if dl.exceeded() {
			return false, nil
		}
		row := i * width
		nextRow := (i + 1) * width
		for j := n - 1; j >= 0; j-- {
			if j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
				return false, nil
			}
			if new[newStart+i] == old[oldStart+j] {
				dp[row+j] = dp[nextRow+j+1] + 1
			} else {
				dp[row+j] = max(dp[nextRow+j], dp[row+j+1])
			}
		}
	}

	emittedAny := false
	i, j := 0, 0
	for i < m && j < n {
		if !emittedAny && j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
			return false, nil
		}
		row := i * width
		oldIdx := oldStart + j
		newIdx := newStart + i

		if new[newIdx] == old[oldIdx] && dp[row+j] == dp[(i+1)*width+j+1]+1 {
			startI, startJ := i, j
			for i < m && j < n {
				if !emittedAny && j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
					return false, nil
				}
				row := i * width
				if new[newStart+i] == old[oldStart+j] && dp[row+j] == dp[(i+1)*width+j+1]+1 {
					i++
					j++
				} else {
					break
				}
			}
			if err := d.Equal(oldStart+startJ, newStart+startI, j-startJ); err != nil {
				return false, err
			}
			emittedAny = true
		} else if dp[(i+1)*width+j] >= dp[row+j+1] {
			startI := i
			insOldIdx := oldStart + j
			for i < m {
				if !emittedAny && j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
					return false, nil
				}
				if j >= n {
					i = m
					break
				}
				row := i * width
				if new[newStart+i] == old[oldStart+j] && dp[row+j] == dp[(i+1)*width+j+1]+1 {
					break
				}
				if dp[(i+1)*width+j] >= dp[row+j+1] {
					i++
				} else {
					break
				}
			}
			if err := d.Insert(insOldIdx, newStart+startI, i-startI); err != nil {
				return false, err
			}
			emittedAny = true
		} else {
			startJ := j
			delNewIdx := newStart + i
			for j < n {
				if !emittedAny && j&(smallSideDeadlineCheckInterval-1) == 0 && dl.exceeded() {
					return false, nil
				}
				if i >= m {
					j = n
					break
				}
				row := i * width
				if new[newStart+i] == old[oldStart+j] && dp[row+j] == dp[(i+1)*width+j+1]+1 {
					break
				}
				if dp[(i+1)*width+j] < dp[row+j+1] {
					j++
				} else {
					break
				}
			}
			if err := d.Delete(oldStart+startJ, j-startJ, delNewIdx); err != nil {
				return false, err
			}
			emittedAny = true
		}
	}

	if j < n {
		if err := d.Delete(oldStart+j, n-j, newStart+i); err != nil {
			return false, err
		}
	}
	if i < m {
		if err := d.Insert(oldStart+j, newStart+i, m-i); err != nil {
			return false, err
		}
	}

	return true, nil
}
