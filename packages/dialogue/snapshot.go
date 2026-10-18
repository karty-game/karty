package dialogue

import "fmt"

func (model *Model) Save() ([]byte, error)    { return model.save(false) }
func (model *Model) SaveRun() ([]byte, error) { return model.save(true) }
func (model *Model) save(run bool) ([]byte, error) {
	if err := model.Validate(); err != nil {
		return nil, err
	}
	magic := "DLS2"
	if run {
		magic = "DLR2"
	}
	w := newRecord(magic, 48*1024)
	for _, s := range []string{model.Speaker, model.Text, model.Scene, model.Portrait, model.Location} {
		w.str(s, 4096)
	}
	if run {
		w.str("", 4096)
	} else {
		w.str(model.Status, 4096)
	}
	w.str(model.Ending, 4096)
	w.boolean(model.Done)
	w.boolean(!run && model.Music)
	w.boolean(!run && model.Ambience)
	w.integer(len(model.Choices), 32)
	for _, c := range model.Choices {
		w.integer(c.ID, 65535)
		w.str(c.Text, 4096)
		w.boolean(c.Enabled)
	}
	w.integer(len(model.History), MaxHistory)
	for _, e := range model.History {
		w.str(e.ID, 4096)
		w.str(e.Speaker, 4096)
		w.str(e.Text, 4096)
	}
	return w.finish()
}
func (model *Model) Restore(data []byte) error { return model.restore(data, false) }

// RestoreRun accepts binary full or run records, preserving current preferences.
func (model *Model) RestoreRun(data []byte) error { return model.restore(data, true) }
func (model *Model) restore(data []byte, run bool) error {
	if len(data) < 8 {
		return fmt.Errorf("invalid dialogue snapshot")
	}
	magic := string(data[:4])
	if magic != "DLS2" && (!run || magic != "DLR2") {
		return fmt.Errorf("incompatible dialogue snapshot")
	}
	if _, err := decodeModel(data, magic, false); err != nil {
		return err
	}
	restored, err := decodeModel(data, magic, true)
	if err != nil {
		return err
	}
	if run {
		restored.Music, restored.Ambience = model.Music, model.Ambience
		restored.Status = ""
	}
	if err := restored.Validate(); err != nil {
		return err
	}
	if restored.Speaker == "Narrator" {
		restored.Speaker = ""
	}
	for i := range restored.History {
		if restored.History[i].Speaker == "Narrator" {
			restored.History[i].Speaker = ""
		}
	}
	*model = restored
	return nil
}
func decodeModel(data []byte, magic string, build bool) (Model, error) {
	r := readRecord(data, magic, 48*1024, build)
	m := Model{
		Speaker:  r.str(4096),
		Text:     r.str(4096),
		Scene:    r.str(4096),
		Portrait: r.str(4096),
		Location: r.str(4096),
		Status:   r.str(4096),
		Ending:   r.str(4096),
		Done:     r.boolean(),
		Music:    r.boolean(),
		Ambience: r.boolean(),
	}
	n := r.count(32, 9)
	for i := 0; i < n; i++ {
		v := Option{ID: r.integer(65535), Text: r.str(4096), Enabled: r.boolean()}
		if build {
			m.Choices = append(m.Choices, v)
		}
	}
	n = r.count(MaxHistory, 12)
	for i := 0; i < n; i++ {
		v := Entry{ID: r.str(4096), Speaker: r.str(4096), Text: r.str(4096)}
		if build {
			m.History = append(m.History, v)
		}
	}
	return m, r.done()
}
