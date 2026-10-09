package easypanel

import (
	"errors"
	"io/fs"
	"path"
)

// projectsDir is the directory of the Easypanel projects under the root of the host, with one directory per project and, in it, one per service.
const projectsDir = "etc/easypanel/projects"

// ReportCollectorService collects the bytes of the directories of the Easypanel services under the root of the host.
type ReportCollectorService struct {
	fsys fs.FS
}

// NewReportCollectorService returns a ReportCollectorService that reads the directories from fsys, the root of the host.
func NewReportCollectorService(fsys fs.FS) *ReportCollectorService {
	return &ReportCollectorService{fsys: fsys}
}

// Collect returns the bytes of the directory of each service of each project in etc/easypanel/projects, none when that directory does not exist.
func (s *ReportCollectorService) Collect() (*Snapshot, error) {
	projects, err := s.directories(projectsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return &Snapshot{}, nil
	}
	if err != nil {
		return nil, err
	}

	sizes := map[string]map[string]int64{}
	for _, project := range projects {
		services, err := s.serviceSizes(path.Join(projectsDir, project))
		if err != nil {
			return nil, err
		}

		sizes[project] = services
	}

	return &Snapshot{Sizes: sizes}, nil
}

// serviceSizes returns the bytes of the directory of each service in projectDir, the directory of a project, by its name.
func (s *ReportCollectorService) serviceSizes(projectDir string) (map[string]int64, error) {
	services, err := s.directories(projectDir)
	if err != nil {
		return nil, err
	}

	sizes := map[string]int64{}
	for _, service := range services {
		size, err := s.size(path.Join(projectDir, service))
		if err != nil {
			return nil, err
		}

		sizes[service] = size
	}

	return sizes, nil
}

// directories returns the names of the directories in dir, skipping its other entries.
func (s *ReportCollectorService) directories(dir string) ([]string, error) {
	entries, err := fs.ReadDir(s.fsys, dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	return names, nil
}

// size returns the bytes of the regular files under dir.
func (s *ReportCollectorService) size(dir string) (int64, error) {
	var size int64
	err := fs.WalkDir(s.fsys, dir, func(_ string, entry fs.DirEntry, err error) error {
		entrySize, err := regularSize(entry, err)
		size += entrySize

		return err
	})

	return size, err
}

// regularSize returns the bytes of entry when it is a regular file and 0 otherwise, failing with err, the error of the walk on entry, or when the info of entry cannot be read.
func regularSize(entry fs.DirEntry, err error) (int64, error) {
	if err != nil {
		return 0, err
	}

	if !entry.Type().IsRegular() {
		return 0, nil
	}

	info, err := entry.Info()
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}
