package tui

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/zydou/portal/internal/conn"
	"github.com/zydou/portal/internal/semver"
	"github.com/zydou/portal/protocol/transfer"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ------------------------------------------------- Shared UI Messages ------------------------------------------------

type ErrorMsg error

type ProgressMsg int

type SecureMsg struct {
	Conn conn.Transfer
}

// StatusMsg carries a status message and the command that follows it.
// The message is rendered as part of the view instead of printed via tea.Println.
type StatusMsg struct {
	Text string
	Cmd  tea.Cmd
}

type TransferStateMessage struct {
	State transfer.MsgType
}

type VersionMsg struct {
	ServerVersion semver.Version
}

// ------------------------------------------------------ Spinners -----------------------------------------------------

var WaitingSpinner = spinner.Spinner{
	Frames: []string{"⠋ ", "⠙ ", "⠹ ", "⠸ ", "⠼ ", "⠴ ", "⠦ ", "⠧ ", "⠇ ", "⠏ "},
	FPS:    time.Second / 12,
}

var PackingSpinner = spinner.Spinner{
	Frames: []string{"┉┉┉", "┅┅┅", "┄┄┄", "┉ ┉", "┅ ┅", "┄ ┄", " ┉ ", " ┉ ", " ┅ ", " ┅ ", " ┄ "},
	FPS:    time.Second / 3,
}

var TransferSpinner = spinner.Spinner{
	Frames: []string{"⇢┄┄", "┄⇢┄", "┄┄⇢", "┄┄┄"},
	FPS:    time.Millisecond * 400,
}

var ReceivingSpinner = spinner.Spinner{
	Frames: []string{"┄┄┄", "┄┄⇠", "┄⇠┄", "⇠┄┄"},
	FPS:    time.Second / 2,
}

// --------------------------------------------------- Shared Helpers --------------------------------------------------

func LogSeparator(width int) string {
	paddedWidth := math.Max(0, float64(width)-2*MARGIN)
	return fmt.Sprintf("%s\n\n",
		BaseStyle.
			Foreground(lipgloss.Color(SECONDARY_COLOR)).
			Render(strings.Repeat("─", int(math.Min(MAX_WIDTH, paddedWidth)))))
}

func TopLevelFilesText(fileNames []string) string {
	// parse top level file names and attach number of subfiles in them
	topLevelFileChildren := make(map[string]int)
	for _, f := range fileNames {
		fileTopPath := strings.Split(f, "/")[0]
		subfileCount, wasPresent := topLevelFileChildren[fileTopPath]
		if wasPresent {
			topLevelFileChildren[fileTopPath] = subfileCount + 1
		} else {
			topLevelFileChildren[fileTopPath] = 0
		}
	}
	// read map into formatted strings
	var topLevelFilesText []string
	for fileName, subFileCount := range topLevelFileChildren {
		formattedFileName := fileName
		if subFileCount > 0 {
			formattedFileName = fmt.Sprintf("%s (%d subfiles)", fileName, subFileCount)
		}
		topLevelFilesText = append(topLevelFilesText, formattedFileName)
	}
	sort.Strings(topLevelFilesText)
	return strings.Join(topLevelFilesText, ", ")
}

// Credits to (legendary Mr. Nilsson): https://yourbasic.org/golang/formatting-byte-size-to-human-readable-format/
func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

// -------------------------------------------------- Shared Commands --------------------------------------------------

// TaskCmd wraps a status message alongside the given command. The message is
// delivered to the model as a [StatusMsg] so it can be rendered as part of the
// View (rather than printed above the screen via tea.Println, which conflicts
// with the v2 renderer's ScreenBuffer).
func TaskCmd(task string, cmd tea.Cmd) tea.Cmd {
	msg := PadText + "• " + task
	return func() tea.Msg {
		return StatusMsg{Text: msg, Cmd: cmd}
	}
}

// RenderStatusLog renders accumulated status messages (each already formatted
// with bullet prefix and padding by [TaskCmd]) as a block for inclusion in a View.
func RenderStatusLog(log []string) string {
	if len(log) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, line := range log {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func QuitCmd() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(SHUTDOWN_PERIOD)
		return tea.Quit()
	}
}

func VersionCmd(ctx context.Context, rendezvousAddr string) tea.Cmd {
	return func() tea.Msg {
		ver, err := semver.GetRendezvousVersion(ctx, rendezvousAddr)
		if err != nil {
			return ErrorMsg(err)
		}
		return VersionMsg{
			ServerVersion: ver,
		}
	}
}

func ErrorCmd(err error) tea.Cmd {
	return TaskCmd(ErrorText(err.Error()), QuitCmd())
}
