package fortify

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	ff "github.com/piper-validation/fortify-client-go/fortify"
	"github.com/piper-validation/fortify-client-go/fortify/artifact_of_project_version_controller"
	"github.com/piper-validation/fortify-client-go/fortify/attribute_of_project_version_controller"
	"github.com/piper-validation/fortify-client-go/fortify/auth_entity_of_project_version_controller"
	"github.com/piper-validation/fortify-client-go/fortify/file_token_controller"
	"github.com/piper-validation/fortify-client-go/fortify/filter_set_of_project_version_controller"
	"github.com/piper-validation/fortify-client-go/fortify/issue_group_of_project_version_controller"
	"github.com/piper-validation/fortify-client-go/fortify/issue_selector_set_of_project_version_controller"
	"github.com/piper-validation/fortify-client-go/fortify/issue_statistics_of_project_version_controller"
	"github.com/piper-validation/fortify-client-go/fortify/project_controller"
	"github.com/piper-validation/fortify-client-go/fortify/project_version_controller"
	"github.com/piper-validation/fortify-client-go/fortify/project_version_of_project_controller"
	"github.com/piper-validation/fortify-client-go/fortify/saved_report_controller"
	"github.com/piper-validation/fortify-client-go/models"

	piperHttp "github.com/SAP/jenkins-library/pkg/http"
	"github.com/SAP/jenkins-library/pkg/log"
	"github.com/SAP/jenkins-library/pkg/piperutils"
	"github.com/sirupsen/logrus"
)

// System is the interface abstraction of a specific SystemInstance
type System interface {
}

// SystemInstance is the specific instance
type SystemInstance struct {
	timeout    time.Duration
	token      string
	serverURL  string
	client     *ff.Fortify
	httpClient *piperHttp.Client
	logger     *logrus.Entry
}

// NewSystemInstance - creates an returns a new SystemInstance
func NewSystemInstance(serverURL, endpoint, authToken string, timeout time.Duration) *SystemInstance {
	schemeHost := strings.Split(serverURL, "://")
	hostEndpoint := strings.Split(schemeHost[1], "/")
	format := strfmt.Default
	dateTimeFormat := models.Iso8601MilliDateTime{}
	format.Add("datetime", &dateTimeFormat, models.IsDateTime)
	clientInstance := ff.NewHTTPClientWithConfig(format, &ff.TransportConfig{
		Host:     hostEndpoint[0],
		Schemes:  []string{schemeHost[0]},
		BasePath: fmt.Sprintf("%v/%v", hostEndpoint[1], endpoint)},
	)
	httpClientInstance := &piperHttp.Client{}
	httpClientOptions := piperHttp.ClientOptions{Token: "FortifyToken " + authToken, Timeout: timeout}
	httpClientInstance.SetOptions(httpClientOptions)

	return NewSystemInstanceForClient(clientInstance, httpClientInstance, serverURL, authToken, timeout)
}

// NewSystemInstanceForClient - creates a new SystemInstance
func NewSystemInstanceForClient(clientInstance *ff.Fortify, httpClientInstance *piperHttp.Client, serverURL, authToken string, requestTimeout time.Duration) *SystemInstance {
	return &SystemInstance{
		timeout:    requestTimeout,
		token:      authToken,
		serverURL:  serverURL,
		client:     clientInstance,
		httpClient: httpClientInstance,
		logger:     log.Entry().WithField("package", "SAP/jenkins-library/pkg/fortify"),
	}
}

// AuthenticateRequest authenticates the request
func (sys *SystemInstance) AuthenticateRequest(req runtime.ClientRequest, formats strfmt.Registry) error {
	req.SetHeaderParam("Authorization", fmt.Sprintf("FortifyToken %v", sys.token))
	return nil
}

// GetProjectByName returns the project identified by the name provided
func (sys *SystemInstance) GetProjectByName(name string) (*models.Project, error) {
	nameParam := fmt.Sprintf("name=%v", name)
	fullText := true
	params := &project_controller.ListProjectParams{Q: &nameParam, Fulltextsearch: &fullText}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.ProjectController.ListProject(params, sys)
	if err != nil {
		return nil, err
	}
	for _, project := range result.GetPayload().Data {
		if *project.Name == name {
			return project, nil
		}
	}
	return nil, fmt.Errorf("Project with name %v not found in backend", name)
}

// GetProjectVersionDetailsByNameAndProjectID returns the project version details of the project version identified by the id
func (sys *SystemInstance) GetProjectVersionDetailsByNameAndProjectID(id int64, name string) (*models.ProjectVersion, error) {
	nameParam := fmt.Sprintf("name=%v", name)
	fullText := true
	params := &project_version_of_project_controller.ListProjectVersionOfProjectParams{ParentID: id, Q: &nameParam, Fulltextsearch: &fullText}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.ProjectVersionOfProjectController.ListProjectVersionOfProject(params, sys)
	if err != nil {
		return nil, err
	}
	for _, projectVersion := range result.GetPayload().Data {
		if *projectVersion.Name == name {
			return projectVersion, nil
		}
	}
	return nil, fmt.Errorf("Project version with name %v not found in for project with ID %v", name, id)
}

// GetProjectVersionAttributesByID returns the project version attributes of the project version identified by the id
func (sys *SystemInstance) GetProjectVersionAttributesByID(id int64) ([]*models.Attribute, error) {
	params := &attribute_of_project_version_controller.ListAttributeOfProjectVersionParams{ParentID: id}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.AttributeOfProjectVersionController.ListAttributeOfProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// CreateProjectVersion creates the project version with the provided details
func (sys *SystemInstance) CreateProjectVersion(version *models.ProjectVersion) (*models.ProjectVersion, error) {
	params := &project_version_controller.CreateProjectVersionParams{Resource: version}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.ProjectVersionController.CreateProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// ProjectVersionCopyFromPartial copies parts of the source project version to the target project version identified by their ids
func (sys *SystemInstance) ProjectVersionCopyFromPartial(sourceID, targetID int64) error {
	enable := true
	settings := models.ProjectVersionCopyPartialRequest{
		ProjectVersionID:            &targetID,
		PreviousProjectVersionID:    &sourceID,
		CopyAnalysisProcessingRules: &enable,
		CopyBugTrackerConfiguration: &enable,
		CopyCurrentStateFpr:         &enable,
		CopyCustomTags:              &enable,
	}
	params := &project_version_controller.CopyProjectVersionParams{Resource: &settings}
	params.WithTimeout(sys.timeout)
	_, err := sys.client.ProjectVersionController.CopyProjectVersion(params, sys)
	if err != nil {
		return err
	}
	return nil
}

// ProjectVersionCopyCurrentState copies the project version state of sourceID into the new project version addressed by targetID
func (sys *SystemInstance) ProjectVersionCopyCurrentState(sourceID, targetID int64) error {
	enable := true
	settings := models.ProjectVersionCopyCurrentStateRequest{
		ProjectVersionID:         &targetID,
		PreviousProjectVersionID: &sourceID,
		CopyCurrentStateFpr:      &enable,
	}
	params := &project_version_controller.CopyCurrentStateForProjectVersionParams{Resource: &settings}
	params.WithTimeout(sys.timeout)
	_, err := sys.client.ProjectVersionController.CopyCurrentStateForProjectVersion(params, sys)
	if err != nil {
		return err
	}
	return nil
}

func (sys *SystemInstance) getAuthEntityOfProjectVersion(id int64) ([]*models.AuthenticationEntity, error) {
	embed := "roles"
	params := &auth_entity_of_project_version_controller.ListAuthEntityOfProjectVersionParams{Embed: &embed, ParentID: id}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.AuthEntityOfProjectVersionController.ListAuthEntityOfProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

func (sys *SystemInstance) updateCollectionAuthEntityOfProjectVersion(id int64, data []*models.AuthenticationEntity) error {
	params := &auth_entity_of_project_version_controller.UpdateCollectionAuthEntityOfProjectVersionParams{ParentID: id, Data: data}
	params.WithTimeout(sys.timeout)
	_, err := sys.client.AuthEntityOfProjectVersionController.UpdateCollectionAuthEntityOfProjectVersion(params, sys)
	if err != nil {
		return err
	}
	return nil
}

// CopyProjectVersionPermissions copies the authentication entity of the project version addressed by sourceID to the one of targetID
func (sys *SystemInstance) CopyProjectVersionPermissions(sourceID, targetID int64) error {
	result, err := sys.getAuthEntityOfProjectVersion(sourceID)
	if err != nil {
		return err
	}
	err = sys.updateCollectionAuthEntityOfProjectVersion(targetID, result)
	if err != nil {
		return err
	}
	return nil
}

func (sys *SystemInstance) updateProjectVersionDetails(id int64, details *models.ProjectVersion) (*models.ProjectVersion, error) {
	params := &project_version_controller.UpdateProjectVersionParams{ID: id, Resource: details}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.ProjectVersionController.UpdateProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// CommitProjectVersion commits the project version with the provided id
func (sys *SystemInstance) CommitProjectVersion(id int64) (*models.ProjectVersion, error) {
	enabled := true
	update := models.ProjectVersion{Committed: &enabled}
	return sys.updateProjectVersionDetails(id, &update)
}

// InactivateProjectVersion inactivates the project version with the provided id
func (sys *SystemInstance) InactivateProjectVersion(id int64) (*models.ProjectVersion, error) {
	enabled := true
	disabled := false
	update := models.ProjectVersion{Committed: &enabled, Active: &disabled}
	return sys.updateProjectVersionDetails(id, &update)
}

// GetArtifactsOfProjectVersion returns the list of artifacts related to the project version addressed with id
func (sys *SystemInstance) GetArtifactsOfProjectVersion(id int64) ([]*models.Artifact, error) {
	scans := "scans"
	params := &artifact_of_project_version_controller.ListArtifactOfProjectVersionParams{ParentID: id, Embed: &scans}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.ArtifactOfProjectVersionController.ListArtifactOfProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// GetFilterSetOfProjectVersionByTitle returns the filter set with the given title related to the project version addressed with id, if no title is provided the default filter set will be returned
func (sys *SystemInstance) GetFilterSetOfProjectVersionByTitle(id int64, title string) (*models.FilterSet, error) {
	params := &filter_set_of_project_version_controller.ListFilterSetOfProjectVersionParams{ParentID: id}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.FilterSetOfProjectVersionController.ListFilterSetOfProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	for _, filterSet := range result.GetPayload().Data {
		if len(title) > 0 && filterSet.Title == title {
			return filterSet, nil
		}
		if len(title) == 0 && filterSet.DefaultFilterSet {
			return filterSet, nil
		}
	}
	return nil, fmt.Errorf("Failed to identify requested filter set or default one")
}

// GetIssueFilterSelectorOfProjectVersionByName returns the groupings with the given names related to the project version addressed with id
func (sys *SystemInstance) GetIssueFilterSelectorOfProjectVersionByName(id int64, names ...string) (*models.IssueFilterSelectorSet, error) {
	params := &issue_selector_set_of_project_version_controller.GetIssueSelectorSetOfProjectVersionParams{ParentID: id}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.IssueSelectorSetOfProjectVersionController.GetIssueSelectorSetOfProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	groupingList := []*models.IssueSelector{}
	if result.GetPayload().Data.GroupBySet != nil {
		for _, group := range result.GetPayload().Data.GroupBySet {
			if piperutils.ContainsString(names, *group.DisplayName) {
				groupingList = append(groupingList, group)
			}
		}
	}
	filterList := []*models.IssueFilterSelector{}
	if result.GetPayload().Data.FilterBySet != nil {
		for _, filter := range result.GetPayload().Data.FilterBySet {
			if piperutils.ContainsString(names, filter.DisplayName) {
				filterList = append(filterList, filter)
			}
		}
	}
	return &models.IssueFilterSelectorSet{GroupBySet: groupingList, FilterBySet: filterList}, nil
}

func (sys *SystemInstance) getIssuesOfProjectVersion(id int64, filter, filterset, groupingtype string) ([]*models.ProjectVersionIssueGroup, error) {
	enable := true
	params := &issue_group_of_project_version_controller.ListIssueGroupOfProjectVersionParams{ParentID: id, Showsuppressed: &enable, Filterset: &filterset, Groupingtype: &groupingtype}
	params.WithTimeout(sys.timeout)
	if len(filter) > 0 {
		params.WithFilter(&filter)
	}
	result, err := sys.client.IssueGroupOfProjectVersionController.ListIssueGroupOfProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// GetProjectIssuesByIDAndFilterSetGroupedByFolder returns issues of the project version addressed with id filtered with the respective set and grouped by folder
func (sys *SystemInstance) GetProjectIssuesByIDAndFilterSetGroupedByFolder(id int64, filterSetTitle string) ([]*models.ProjectVersionIssueGroup, error) {
	filterset := ""
	filterSets, _ := sys.GetFilterSetOfProjectVersionByTitle(id, filterSetTitle)
	if filterSets != nil {
		filterset = filterSets.GUID
	}
	groupingtype := ""
	filterSelector, _ := sys.GetIssueFilterSelectorOfProjectVersionByName(id, "Folder")
	if filterSelector != nil {
		groupingtype = *filterSelector.GroupBySet[0].GUID
	}

	result, err := sys.getIssuesOfProjectVersion(id, "", filterset, groupingtype)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (sys *SystemInstance) getFilterValueByName(filterSets *models.FilterSet, name string) string {
	for _, folder := range filterSets.Folders {
		if folder.Name == name {
			return folder.GUID
		}
	}
	return ""
}

// GetProjectIssuesByIDAndFilterSetGroupedByCategory returns issues of the project version addressed with id filtered with the respective set and grouped by category
func (sys *SystemInstance) GetProjectIssuesByIDAndFilterSetGroupedByCategory(id int64, filterSetTitle string) ([]*models.ProjectVersionIssueGroup, error) {
	filterset := ""
	filterSets, _ := sys.GetFilterSetOfProjectVersionByTitle(id, filterSetTitle)
	if filterSets != nil {
		filterset = filterSets.GUID
	}
	groupingtype := ""
	filterSelector, _ := sys.GetIssueFilterSelectorOfProjectVersionByName(id, "Category")
	if filterSelector != nil {
		groupingtype = *filterSelector.GroupBySet[0].GUID
	}

	result, err := sys.getIssuesOfProjectVersion(id, sys.getFilterValueByName(filterSets, "Spot Checks of Each Category"), filterset, groupingtype)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetProjectIssuesByIDAndFilterSetGroupedByAnalysis returns issues of the project version addressed with id filtered with the respective set and grouped by analysis
func (sys *SystemInstance) GetProjectIssuesByIDAndFilterSetGroupedByAnalysis(id int64, filterSetTitle string) ([]*models.ProjectVersionIssueGroup, error) {
	filterset := ""
	filterSets, _ := sys.GetFilterSetOfProjectVersionByTitle(id, filterSetTitle)
	if filterSets != nil {
		filterset = filterSets.GUID
	}
	groupingtype := ""
	filterSelector, _ := sys.GetIssueFilterSelectorOfProjectVersionByName(id, "Analysis")
	if filterSelector != nil {
		groupingtype = *filterSelector.GroupBySet[0].GUID
	}

	result, err := sys.getIssuesOfProjectVersion(id, "", filterset, groupingtype)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetIssueStatisticsOfProjectVersion returns the issue statistics related to the project version addressed with id
func (sys *SystemInstance) GetIssueStatisticsOfProjectVersion(id int64) ([]*models.IssueStatistics, error) {
	params := &issue_statistics_of_project_version_controller.ListIssueStatisticsOfProjectVersionParams{ParentID: id}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.IssueStatisticsOfProjectVersionController.ListIssueStatisticsOfProjectVersion(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// GenerateQGateReport returns the issue statistics related to the project version addressed with id
func (sys *SystemInstance) GenerateQGateReport(projectID, projectVersionID int64, projectName, projectVersionName, reportFormat string) (*models.SavedReport, error) {
	paramIdentifier := "projectVersionId"
	paramReportDefinitionID := int64(18)
	paramType := "SINGLE_PROJECT"
	paramName := "Q-gate-report"
	reportType := "PORTFOLIO"
	inputReportParameters := []*models.InputReportParameter{&models.InputReportParameter{Name: &paramName, Identifier: &paramIdentifier, ParamValue: projectVersionID, Type: &paramType}}
	reportProjectVersions := []*models.ReportProjectVersion{&models.ReportProjectVersion{ID: projectVersionID, Name: projectVersionName}}
	reportProjects := []*models.ReportProject{&models.ReportProject{ID: projectID, Name: projectName, Versions: reportProjectVersions}}
	report := models.SavedReport{Name: "FortifyReport", Type: &reportType, ReportDefinitionID: &paramReportDefinitionID, Format: &reportFormat, Projects: reportProjects, InputReportParameters: inputReportParameters}
	params := &saved_report_controller.CreateSavedReportParams{Resource: &report}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.SavedReportController.CreateSavedReport(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// GetReportDetails returns the details of the report addressed with id
func (sys *SystemInstance) GetReportDetails(id int64) (*models.SavedReport, error) {
	params := &saved_report_controller.ReadSavedReportParams{ID: id}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.SavedReportController.ReadSavedReport(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// InvalidateFileTokens invalidates the file tokens
func (sys *SystemInstance) InvalidateFileTokens() error {
	params := &file_token_controller.MultiDeleteFileTokenParams{}
	params.WithTimeout(sys.timeout)
	_, err := sys.client.FileTokenController.MultiDeleteFileToken(params, sys)
	return err
}

func (sys *SystemInstance) getFileToken(tokenType string) (*models.FileToken, error) {
	token := models.FileToken{FileTokenType: &tokenType}
	params := &file_token_controller.CreateFileTokenParams{Resource: &token}
	params.WithTimeout(sys.timeout)
	result, err := sys.client.FileTokenController.CreateFileToken(params, sys)
	if err != nil {
		return nil, err
	}
	return result.GetPayload().Data, nil
}

// GetFileUploadToken returns the token upload result files
func (sys *SystemInstance) GetFileUploadToken() (*models.FileToken, error) {
	return sys.getFileToken("UPLOAD")
}

// GetFileDownloadToken returns the token to download result files
func (sys *SystemInstance) GetFileDownloadToken() (*models.FileToken, error) {
	return sys.getFileToken("DOWNLOAD")
}

// GetReportFileToken returns the token to download report files
func (sys *SystemInstance) GetReportFileToken() (*models.FileToken, error) {
	return sys.getFileToken("REPORT_FILE")
}

// UploadFile uploads a file to the fortify backend
func (sys *SystemInstance) UploadFile(endpoint, file string, projectVersionID int64) error {
	token, err := sys.GetFileUploadToken()
	if err != nil {
		return err
	}
	defer sys.InvalidateFileTokens()

	header := http.Header{}
	header.Add("Cache-Control", "no-cache, no-store, must-revalidate")
	header.Add("Pragma", "no-cache")
	formFields := map[string]string{}
	formFields["mat"] = token.Token
	formFields["entityId"] = fmt.Sprintf("%v", projectVersionID)
	_, err = sys.httpClient.UploadFileForm(fmt.Sprintf("%v%v", sys.serverURL, endpoint), file, "file", formFields, header, nil)
	return err
}

// DownloadFile downloads a file from Fortify backend
func (sys *SystemInstance) downloadFile(endpoint, method, acceptType, downloadToken string, projectVersionID int64) ([]byte, error) {
	header := http.Header{}
	header.Add("Cache-Control", "no-cache, no-store, must-revalidate")
	header.Add("Pragma", "no-cache")
	header.Add("Accept", acceptType)
	header.Add("Content-Type", "application/form-data")
	body := url.Values{
		"mat": {downloadToken},
		"id":  {fmt.Sprintf("%v", projectVersionID)},
	}
	var response *http.Response
	var err error
	if method == http.MethodGet {
		response, err = sys.httpClient.SendRequest(method, fmt.Sprintf("%v%v?%v", sys.serverURL, endpoint, body.Encode()), nil, header, nil)
	} else {
		response, err = sys.httpClient.SendRequest(method, fmt.Sprintf("%v%v", sys.serverURL, endpoint), strings.NewReader(body.Encode()), header, nil)
	}
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("Error reading the response data %s", err)
	}
	return data, nil
}

// DownloadReportFile downloads a report file from Fortify backend
func (sys *SystemInstance) DownloadReportFile(endpoint string, projectVersionID int64) ([]byte, error) {
	token, err := sys.GetReportFileToken()
	if err != nil {
		return nil, fmt.Errorf("Error fetching report download token %s", err)
	}
	defer sys.InvalidateFileTokens()
	data, err := sys.downloadFile(endpoint, http.MethodGet, "application/octet-stream", token.Token, projectVersionID)
	if err != nil {
		return nil, fmt.Errorf("Error downloading report file %s", err)
	}
	return data, nil
}

// DownloadResultFile downloads a report file from Fortify backend
func (sys *SystemInstance) DownloadResultFile(endpoint string, projectVersionID int64) ([]byte, error) {
	token, err := sys.GetFileDownloadToken()
	if err != nil {
		return nil, fmt.Errorf("Error fetching result file download token %s", err)
	}
	defer sys.InvalidateFileTokens()
	data, err := sys.downloadFile(endpoint, http.MethodPost, "application/zip", token.Token, projectVersionID)
	if err != nil {
		return nil, fmt.Errorf("Error downloading result file %s", err)
	}
	return data, nil
}
