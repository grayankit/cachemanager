package disk

type Totals struct {
	Safe          int64  `json:"safe"`
	Expensive     int64  `json:"expensive"`
	Yours         int64  `json:"yours"`
	SafeText      string `json:"safeText"`
	ExpensiveText string `json:"expensiveText"`
	YoursText     string `json:"yoursText"`
}

type Store struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Path        string `json:"path,omitempty"`
	Size        int64  `json:"size"`
	Reclaimable int64  `json:"reclaimable"`
	SizeText    string `json:"sizeText"`
	ReclaimText string `json:"reclaimText"`
	Detail      string `json:"detail"`
	Icon        string `json:"icon"`
	CanClean    bool   `json:"canClean"`
	NeedsRoot   bool   `json:"needsRoot"`
}

type Report struct {
	Filesystem FS      `json:"filesystem"`
	Totals     Totals  `json:"totals"`
	Stores     []Store `json:"stores"`
	RootHelper bool    `json:"rootHelper"`
	ScannedAt  string  `json:"scannedAt"`
}

type Result struct {
	ID        string   `json:"id"`
	Applied   bool     `json:"applied"`
	DryRun    bool     `json:"dryRun"`
	Reclaimed int64    `json:"reclaimed"`
	Detail    string   `json:"detail"`
	Files     []string `json:"files,omitempty"`
	Log       string   `json:"log,omitempty"`
	Error     string   `json:"error,omitempty"`
	Results   []Result `json:"results,omitempty"`
}

func finishStore(s Store) Store {
	s.SizeText = Format(s.Size)
	if s.Reclaimable > 0 {
		s.ReclaimText = "frees " + Format(s.Reclaimable)
	} else {
		s.ReclaimText = s.SizeText
	}
	return s
}
