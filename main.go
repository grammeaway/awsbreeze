package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"strconv"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	awsRSSURL  = "https://aws.amazon.com/about-aws/whats-new/recent/feed/"
	cacheFileName = "awsbreeze/seen.json"
	oldCacheFileName = ".awsbreeze.json"
)

// RSS structures
type RSS struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Items []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// Application state
type Config struct {
	LastSeen map[string]bool `json:"last_seen"`
	LastRun  time.Time       `json:"last_run"`
}

type NewsItem struct {
	Title       string
	Link        string
	Description string
	PubDate     time.Time
	GUID        string
	IsNew       bool
}

func (i NewsItem) FilterValue() string { return i.Title }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 3 }
func (d itemDelegate) Spacing() int                            { return 1 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(NewsItem)
	if !ok {
		return
	}

	var title, desc string
	isSelected := index == m.Index()

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	newStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)

	if i.IsNew {
		title = newStyle.Render("● " + i.Title)
	} else {
		title = titleStyle.Render(i.Title)
	}

	desc = descStyle.Render(strings.TrimSpace(stripHTML(i.Description)))
	if len(desc) > 80 {
		desc = desc[:77] + "..."
	}

	date := dateStyle.Render(i.PubDate.Format("Jan 2, 2006"))

	if isSelected {
		selectedStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1)
		
		fmt.Fprint(w, selectedStyle.Render(fmt.Sprintf("%s\n%s\n%s", title, desc, date)))
	} else {
		fmt.Fprintf(w, "%s\n%s\n%s", title, desc, date)
	}
}

type model struct {
	list        list.Model
	items       []NewsItem
	config      Config
	loading     bool
	err         error
	filterInput textinput.Model
	filtering   bool
	filterDays  int
	showHelp    bool
}

type fetchedMsg []NewsItem
type errMsg error

func initialModel() model {
	config := loadConfig()
	
	filterInput := textinput.New()
	filterInput.Placeholder = "Enter number of days (e.g., 7)"
	filterInput.Width = 20

	items := []list.Item{}
	l := list.New(items, itemDelegate{}, 0, 0)
	l.Title = "awsbreeze - AWS What's New"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Padding(0, 1).
		Bold(true)

	return model{
		list:        l,
		config:      config,
		loading:     true,
		filterInput: filterInput,
		filterDays:  0,
	}
}

func (m model) Init() tea.Cmd {
	return fetchNews
}

func fetchNews() tea.Msg {
	resp, err := http.Get(awsRSSURL)
	if err != nil {
		return errMsg(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errMsg(err)
	}

	var rss RSS
	err = xml.Unmarshal(body, &rss)
	if err != nil {
		return errMsg(err)
	}

	var items []NewsItem
	for _, item := range rss.Channel.Items {
		pubDate := parseAWSDate(item.PubDate)

		items = append(items, NewsItem{
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			PubDate:     pubDate,
			GUID:        item.GUID,
		})
	}

	// Sort by date (newest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].PubDate.After(items[j].PubDate)
	})

	return fetchedMsg(items)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 3)
		return m, nil

	case fetchedMsg:
		m.loading = false
		m.items = []NewsItem(msg)
		
		// Mark new items
		for i := range m.items {
			_, seen := m.config.LastSeen[m.items[i].GUID]
			m.items[i].IsNew = !seen && m.items[i].PubDate.After(m.config.LastRun)
		}
		
		m.applyFilters()
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter":
				days := parseDays(m.filterInput.Value())
				m.filterDays = days
				m.filtering = false
				m.applyFilters()
				return m, nil
			case "esc":
				m.filtering = false
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.saveConfig()
			return m, tea.Quit

		case "enter":
			if len(m.list.Items()) > 0 {
				selected := m.list.SelectedItem().(NewsItem)
				m.markAsSeen(selected.GUID)
				return m, openURL(selected.Link)
			}

		case "r":
			m.loading = true
			return m, fetchNews

		case "f":
			m.filtering = true
			m.filterInput.Focus()
			return m, nil

		case "c":
			m.filterDays = 0
			m.applyFilters()
			return m, nil

		case "n":
			m.markAllAsSeen()
			m.applyFilters()
			return m, nil

		case "h":
			m.showHelp = !m.showHelp
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) applyFilters() {
	var filteredItems []list.Item
	
	for _, item := range m.items {
		if m.filterDays > 0 {
			daysDiff := int(time.Since(item.PubDate).Hours() / 24)
			if daysDiff > m.filterDays {
				continue
			}
		}
		filteredItems = append(filteredItems, item)
	}
	
	m.list.SetItems(filteredItems)
	
	// Update title with filter info
	title := "AWS What's New"
	if m.filterDays > 0 {
		title += fmt.Sprintf(" (Last %d days)", m.filterDays)
	}
	m.list.Title = title
}

func (m *model) markAsSeen(guid string) {
	if m.config.LastSeen == nil {
		m.config.LastSeen = make(map[string]bool)
	}
	m.config.LastSeen[guid] = true
	
	// Update the item in our list
	for i := range m.items {
		if m.items[i].GUID == guid {
			m.items[i].IsNew = false
			break
		}
	}
	m.applyFilters()
}

func (m *model) markAllAsSeen() {
	if m.config.LastSeen == nil {
		m.config.LastSeen = make(map[string]bool)
	}
	
	for i := range m.items {
		m.config.LastSeen[m.items[i].GUID] = true
		m.items[i].IsNew = false
	}
}

func parseDays(input string) int {
	if input == "" {
		return 0
	}
	
	days, err := strconv.Atoi(input)
	if err != nil || days < 0 {
		return 0
	}
	return days
}

func (m *model) saveConfig() {
	m.config.LastRun = time.Now()
	
	// Clean up old entries to keep config file lightweight
	m.cleanupConfig()
	
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	
	configPath := filepath.Join(cacheDir, cacheFileName)
	data, err := json.Marshal(m.config)
	if err != nil {
		return
	}
	
	os.WriteFile(configPath, data, 0644)
}

func (m *model) cleanupConfig() {
	if m.config.LastSeen == nil {
		return
	}
	
	// Keep only entries from items we still have (current RSS feed)
	// This automatically removes old entries that are no longer in the feed
	currentGUIDs := make(map[string]bool)
	for _, item := range m.items {
		currentGUIDs[item.GUID] = true
	}
	
	// Create new map with only current items
	newLastSeen := make(map[string]bool)
	for guid, seen := range m.config.LastSeen {
		if currentGUIDs[guid] {
			newLastSeen[guid] = seen
		}
	}
	
	m.config.LastSeen = newLastSeen
}

func (m model) View() string {
	if m.loading {
		return "Fetching AWS news...\n\nPress 'q' to quit"
	}
	
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'r' to retry or 'q' to quit", m.err)
	}
	
	view := m.list.View()
	
	if m.filtering {
		filterView := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			Render(fmt.Sprintf("Filter by days:\n%s\n\nPress Enter to apply, Esc to cancel", m.filterInput.View()))
		
		view = lipgloss.JoinVertical(lipgloss.Left, view, filterView)
	}
	
	statusLine := "Press 'h' for help"
	if m.showHelp {
		help := `
Controls:
  ↑/↓ or j/k  - Navigate items
  Enter       - Open selected item in browser
  r           - Refresh news
  f           - Filter by date (days)
  c           - Clear all filters
  n           - Mark all as seen (clear new indicators)
  h           - Toggle this help
  q           - Quit

● Green dots indicate new items since last run
`
		statusLine = help
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, view, statusLine)
}

func loadConfig() Config {
	if !cacheDirExists() {
		if err := createCacheDir(); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating cache directory: %v\n", err)
			return Config{LastSeen: make(map[string]bool)}
		}
	}
	if oldCacheExists() {
		moveAndCleanupOldCache()
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return Config{LastSeen: make(map[string]bool)}
	}
	
	cachePath := filepath.Join(cacheDir, cacheFileName)
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return Config{LastSeen: make(map[string]bool)}
	}
	
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return Config{LastSeen: make(map[string]bool)}
	}
	
	if config.LastSeen == nil {
		config.LastSeen = make(map[string]bool)
	}
	
	return config
}

func cacheDirExists() bool {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return false
	}
	
	// Check if the cache directory exists
	_, err = os.Stat(filepath.Join(cacheDir, "awsbreeze"))
	return !os.IsNotExist(err)
}

func oldCacheExists() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	oldCachePath := filepath.Join(homeDir, oldCacheFileName)
	_, err = os.Stat(oldCachePath)
	return !os.IsNotExist(err)
}

func createCacheDir() error {
	// Check if the cache directory exists
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	// Create the cache directory if it doesn't exist
	if _, err := os.Stat(filepath.Join(cacheDir, "awsbreeze")); os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Join(cacheDir, "awsbreeze"), os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create cache directory: %w", err)
		}
	}
	return nil
}

func moveAndCleanupOldCache() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	oldCachePath := filepath.Join(homeDir, oldCacheFileName)
	newCacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	newCachePath := filepath.Join(newCacheDir, cacheFileName)
	if _, err := os.Stat(oldCachePath); os.IsNotExist(err) {
		return // Old cache file doesn't exist, nothing to do
	}

	// Ensure the cache directory exists
	err = createCacheDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cache directory: %v\n", err)
		return
	}
	// Move old cache file to new location
	err = os.Rename(oldCachePath, newCachePath)
	if err != nil {
		if os.IsExist(err) {
			// If the new cache file already exists, we can ignore the error
			return
		}
		fmt.Fprintf(os.Stderr, "Error moving old cache file: %v\n", err)
		return
	}
	// Remove the old cache file if it exists
	if _, err := os.Stat(oldCachePath); err == nil {
		err = os.Remove(oldCachePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error removing old cache file: %v\n", err)
		}
	}

	return
}
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			return nil
		}
		cmd.Start()
		return nil
	}
}

func stripHTML(s string) string {
	// Simple HTML tag removal
	result := ""
	inTag := false
	
	for _, r := range s {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result += string(r)
		}
	}
	
	return strings.TrimSpace(result)
}

func parseAWSDate(dateStr string) time.Time {
	// Try common RSS date formats
	formats := []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",  // RFC 1123 with timezone
		"Mon, 02 Jan 2006 15:04:05 MST",    // RFC 1123 with named timezone
		"Mon, 02 Jan 2006 15:04:05 GMT",    // RFC 1123 GMT
		"2006-01-02T15:04:05-07:00",        // RFC 3339
		"2006-01-02T15:04:05Z",             // RFC 3339 UTC
		"2006-01-02 15:04:05",              // Simple format
		"Jan 02, 2006 15:04:05",            // Alternative format
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}
	
	// If all parsing fails, return current time as fallback
	// but log the issue for debugging
	fmt.Fprintf(os.Stderr, "Warning: Could not parse date '%s', using current time\n", dateStr)
	return time.Now()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
