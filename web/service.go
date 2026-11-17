package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	repo       JobRepository
	prospects  ProspectRepository
	dataFolder string
}

type jobCounter interface {
	Count(context.Context, SelectParams) (int, error)
}

func NewService(repo JobRepository, dataFolder string) *Service {
	prospects, _ := repo.(ProspectRepository)
	return &Service{
		repo:       repo,
		prospects:  prospects,
		dataFolder: dataFolder,
	}
}

func (s *Service) Create(ctx context.Context, job *Job) error {
	return s.repo.Create(ctx, job)
}

func (s *Service) All(ctx context.Context) ([]Job, error) {
	return s.repo.Select(ctx, SelectParams{})
}

// ListJobs returns a page of jobs, clamping invalid page and limit values.
func (s *Service) ListJobs(ctx context.Context, page, limit int) (JobPage, error) {
	var ans JobPage

	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limit = 20
	}

	total, err := s.countJobs(ctx, SelectParams{})
	if err != nil {
		return ans, err
	}

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * limit
	params := SelectParams{
		Limit:  limit,
		Offset: offset,
	}

	jobs, err := s.repo.Select(ctx, params)
	if err != nil {
		return ans, err
	}

	ans = JobPage{
		Jobs:        jobs,
		CurrentPage: page,
		TotalPages:  totalPages,
		Total:       total,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		PrevPage:    page - 1,
		NextPage:    page + 1,
		HasPages:    totalPages > 1,
	}

	return ans, nil
}

func (s *Service) countJobs(ctx context.Context, params SelectParams) (int, error) {
	if counter, ok := s.repo.(jobCounter); ok {
		return counter.Count(ctx, params)
	}

	jobs, err := s.repo.Select(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("select jobs for count: %w", err)
	}

	return len(jobs), nil
}

func (s *Service) Get(ctx context.Context, id string) (Job, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	datapath, err := s.csvPath(id)
	if err != nil {
		return err
	}

	if _, err := os.Stat(datapath); err == nil {
		if err := os.Remove(datapath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) Update(ctx context.Context, job *Job) error {
	return s.repo.Update(ctx, job)
}

func (s *Service) SelectPending(ctx context.Context) ([]Job, error) {
	return s.repo.Select(ctx, SelectParams{Status: StatusPending, Limit: 1})
}

func (s *Service) ListProspectsForJob(ctx context.Context, id string) (map[string]ProspectState, error) {
	if s.prospects == nil {
		return nil, errors.New("prospect storage is unavailable")
	}
	places, err := s.GetPlaces(ctx, id)
	if errors.Is(err, ErrPlacesNotFound) {
		return map[string]ProspectState{}, nil
	}
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(places))
	for _, place := range places {
		if place.Key != "" {
			keys = append(keys, place.Key)
		}
	}
	return s.prospects.ListProspects(ctx, keys)
}

func (s *Service) SetProspectStage(ctx context.Context, key string, stage ProspectStage) (ProspectState, error) {
	if !ValidProspectKey(key) || !stage.Valid() {
		return ProspectState{}, errors.New("invalid prospect key or stage")
	}
	if s.prospects == nil {
		return ProspectState{}, errors.New("prospect storage is unavailable")
	}
	return s.prospects.SetProspectStage(ctx, key, stage)
}

func (s *Service) AddProspectTouch(ctx context.Context, key string, stage ProspectStage, message string) (ProspectTouch, error) {
	if !ValidProspectKey(key) || !stage.Valid() || strings.TrimSpace(message) == "" || len(message) > 2000 {
		return ProspectTouch{}, errors.New("invalid prospect touch")
	}
	if s.prospects == nil {
		return ProspectTouch{}, errors.New("prospect storage is unavailable")
	}
	return s.prospects.AddProspectTouch(ctx, key, stage, message)
}

func (s *Service) ListProspectTouches(ctx context.Context, key string) ([]ProspectTouch, error) {
	if !ValidProspectKey(key) {
		return nil, errors.New("invalid prospect key")
	}
	if s.prospects == nil {
		return nil, errors.New("prospect storage is unavailable")
	}
	return s.prospects.ListProspectTouches(ctx, key)
}

// csvPath returns the on-disk path of a job's CSV output, rejecting ids that
// could escape the data folder.
func (s *Service) csvPath(id string) (string, error) {
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return "", fmt.Errorf("invalid file name")
	}

	return filepath.Join(s.dataFolder, id+".csv"), nil
}

func (s *Service) GetCSV(_ context.Context, id string) (string, error) {
	datapath, err := s.csvPath(id)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(datapath); os.IsNotExist(err) {
		return "", fmt.Errorf("csv file not found for job %s", id)
	}

	return datapath, nil
}
