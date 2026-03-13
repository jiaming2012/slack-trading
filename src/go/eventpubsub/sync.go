package eventpubsub

type Proc func(chan error)

type SyncProcessItem struct {
	Fn    Proc
	Error chan error
}

type SyncProcess struct {
	fn []SyncProcessItem
}

func (s *SyncProcess) Add(process Proc) {
	s.fn = append(s.fn, SyncProcessItem{
		Fn:    process,
		Error: make(chan error),
	})
}

func (s *SyncProcess) Run() error {
	for _, p := range s.fn {
		p.Fn(p.Error)
		err := <-p.Error
		if err != nil {
			return err
		}
	}

	return nil
}
