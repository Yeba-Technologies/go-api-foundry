package main

// Command: generate_domain.go
//
// Description:
// This CLI command helps automate the creation of a new domain/module within the application
// by generating a directory structure and boilerplate files for a Go domain: repository.go,
// service.go, and controller.go. It prompts the user for a domain name, then generates the
// relevant files with appropriate templates, placing them in domain/<domain>.
//
// Usage:
//   make generate-domain
//   # Then follow the prompt to enter your domain name.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const domainDir = "domain/"

func GenerateDomain() {
	fmt.Println("Enter the name of your domain please: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	domainName := strings.TrimSpace(scanner.Text())

	if domainName == "" {
		fmt.Println("unable to create domain, invalid input")
		return
	}

	domainPath := filepath.Join(domainDir, domainName)

	if _, err := os.Stat(domainPath); !os.IsNotExist(err) {
		fmt.Println("Error: Domain already exists. Ignoring creation.")
		return
	}

	if err := os.MkdirAll(domainPath, os.ModePerm); err != nil {
		fmt.Println("Error creating domain: ", err)
		return
	}

	files := map[string]string{
		"repository.go": repoTemplate(domainName),
		"service.go":    serviceTemplate(domainName),
		"controller.go": controllerTemplate(domainName),
		"dto.go":        dtoTemplate(domainName),
	}

	for filename, content := range files {
		filepath := filepath.Join(domainPath, filename)
		if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
			fmt.Println("Error creating file:", err)
		}
	}

	fmt.Println("✅ Domain", domainName, "created successfully!")
	title := cases.Title(language.English).String(domainName)
	fmt.Println("  ===> Next steps:")
	fmt.Println("   1) Create the database model in internal/models/")
	fmt.Printf("      type %s struct {\n", title)
	fmt.Println("          gorm.Model")
	fmt.Println("          // Add your fields here")
	fmt.Println("      }")
	fmt.Println("   2) Register the model in internal/models/main.go ModelRegistry")
	fmt.Println("   3) Implement repository, service, and handlers in the generated files")
	fmt.Println("   4) Register the domain controller in domain/main.go's SetupCoreDomain function:")
	fmt.Printf("      appConfig.RouterService.MountController(%s.New%sController(appConfig.DB, appConfig.Logger))\n", domainName, title)
}

func repoTemplate(domain string) string {
	title := cases.Title(language.English).String(domain)
	return fmt.Sprintf(`package %s

import (
	"context"
	"errors"

	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
	"gorm.io/gorm"
)

// %sRepository defines the data access layer for %s domain
type %sRepository interface {
	// Create persists a new %s entry to the database
	Create(ctx context.Context, entry *models.%s) (*models.%s, error)
	// FindByID retrieves a %s entry by its unique ID
	FindByID(ctx context.Context, id uint) (*models.%s, error)
}

type %sRepository struct {
	db *gorm.DB
}

// New%sRepository creates a new instance of %sRepository
func New%sRepository(db *gorm.DB) %sRepository {
	return &%sRepository{db: db}
}

func (r *%sRepository) Create(ctx context.Context, entry *models.%s) (*models.%s, error) {
	if err := r.db.WithContext(ctx).Create(entry).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, apperrors.NewConflictError("%s entry already exists", err)
		}
		return nil, apperrors.NewDatabaseError("unable to create %s entry", err)
	}
	return entry, nil
}

func (r *%sRepository) FindByID(ctx context.Context, id uint) (*models.%s, error) {
	var entry models.%s
	if err := r.db.WithContext(ctx).First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("%s entry not found", err)
		}
		return nil, apperrors.NewDatabaseError("failed to fetch %s entry", err)
	}
	return &entry, nil
}

// isDuplicateKey checks if the error is a duplicate key constraint violation
func isDuplicateKey(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) || apperrors.IsDuplicateKeyError(err)
}
`,
		domain,               // package name
		title, domain, title, // interface comments
		domain, title, title, // Create method
		domain, title, // FindByID method
		parseText(domain, false), // struct name
		title, title,             // New function
		title, title, // constructor return
		parseText(domain, false),               // receiver
		parseText(domain, false), title, title, // Create implementation
		domain, domain, // Create error messages
		parseText(domain, false), title, title, // FindByID implementation
		domain, domain, // FindByID error messages
	)
}

func serviceTemplate(domain string) string {
	title := cases.Title(language.English).String(domain)
	return fmt.Sprintf(`package %s

import (
	"context"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
)

// %sService defines the business logic layer for %s domain
type %sService interface {
	// Create creates a new %s entry based on the provided request
	Create(ctx context.Context, req *Create%sRequest) (*%sResponse, error)

	// FindByID retrieves a %s entry by its unique ID
	FindByID(ctx context.Context, id uint) (*%sResponse, error)
}

type %sService struct {
	logger     *log.Logger
	repository %sRepository
}

func New%sService(logger *log.Logger, repository %sRepository) %sService {
	return &%sService{
		logger:     logger,
		repository: repository,
	}
}

func (s *%sService) Create(ctx context.Context, req *Create%sRequest) (*%sResponse, error) {
	logger := log.GetLoggerInstanceFromContext(ctx, s.logger)

	if req == nil {
		logger.Error("Create received empty request")
		return nil, apperrors.NewInvalidRequestError("request cannot be nil", nil)
	}

	// Add business validation logic here

	model := To%sModel(req)
	entry, err := s.repository.Create(ctx, model)
	if err != nil {
		logger.Error("Failed to create %s entry", "error", err)
		return nil, err
	}

	response := To%sResponse(entry)
	return &response, nil
}

func (s *%sService) FindByID(ctx context.Context, id uint) (*%sResponse, error) {
	logger := log.GetLoggerInstanceFromContext(ctx, s.logger)

	if id == 0 {
		logger.Error("FindByID received invalid ID")
		return nil, apperrors.NewInvalidRequestError("invalid entry ID", nil)
	}

	entry, err := s.repository.FindByID(ctx, id)
	if err != nil {
		logger.Error("Failed to find %s entry", "id", id, "error", err)
		return nil, err
	}

	response := To%sResponse(entry)
	return &response, nil
}
`,
		domain,               // package name
		title, domain, title, // interface comments
		domain, title, title, // Create method
		domain, title, // FindByID method
		parseText(domain, false), // struct name
		title,                    // struct repository field
		title, title,             // New function
		title, title, // constructor return
		parseText(domain, false), title, title, // Create implementation
		title, domain, // Create body
		title,                           // ToResponse call
		parseText(domain, false), title, // FindByID implementation
		domain, title, // FindByID body
	)
}

func controllerTemplate(domain string) string {
	title := cases.Title(language.English).String(domain)
	return fmt.Sprintf(`package %s

import (
	"github.com/Yeba-Technologies/go-api-foundry/config/router"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
	"gorm.io/gorm"
)

// New%sController creates and returns a versioned RESTController for the %s domain
func New%sController(db *gorm.DB, logger *log.Logger) *router.RESTController {
	return router.NewVersionedRESTController(
		"%sController",
		"v1",
		"/%s",
		func(rs *router.RouterService, c *router.RESTController) {
			repository := New%sRepository(db)
			service := New%sService(logger, repository)

			// Register handlers
			rs.AddPostHandler(c, "", createHandler(service))
			rs.AddGetHandler(c, "/:id", getByIDHandler(service))
		},
	)
}

func createHandler(service %sService) router.HandlerFunction {
	return func(ctx *router.RequestContext) *router.ServiceResult {
		logger := router.GetLogger(ctx)

		var req Create%sRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			logger.Error("Failed to bind request", "error", err)

			validationErrors := apperrors.FormatValidationErrors(err, &req)
			if len(validationErrors) > 0 {
				return router.BadRequestResult("Invalid request payload", validationErrors)
			}

			return router.BadRequestResult("Invalid request body", nil)
		}

		response, err := service.Create(ctx.Request.Context(), &req)
		if err != nil {
			return router.ErrorResult(
				apperrors.HTTPStatusCode(err),
				apperrors.GetHumanReadableMessage(err),
				nil,
			)
		}

		return router.CreatedResult(response, "%s entry")
	}
}

func getByIDHandler(service %sService) router.HandlerFunction {
	return func(ctx *router.RequestContext) *router.ServiceResult {
		id, errResult := router.ParseIDParam(ctx, "id")
		if errResult != nil {
			return errResult
		}

		response, err := service.FindByID(ctx.Request.Context(), id)
		if err != nil {
			return router.ErrorResult(
				apperrors.HTTPStatusCode(err),
				apperrors.GetHumanReadableMessage(err),
				nil,
			)
		}

		return router.OKResult(response, "%s entry retrieved successfully")
	}
}
`,
		domain,               // package name
		title, domain, title, // New controller function
		title, domain, // controller config
		title, title, // wire up
		title, // createHandler
		title, // Create request
		title, // Created result
		title, // getByIDHandler
		title, // OKResult
	)
}

func dtoTemplate(domain string) string {
	title := cases.Title(language.English).String(domain)
	return fmt.Sprintf(`package %s

import (
	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/constants"
)

// Create%sRequest defines the structure for creating a new %s entry
type Create%sRequest struct {
	// Add your request fields here with validation tags
	// Example: Name string `+"`json:\"name\" binding:\"required\"`"+`
}

// %sResponse defines the structure for %s responses
type %sResponse struct {
	ID        uint   `+"`json:\"id\"`"+`
	CreatedAt string `+"`json:\"created_at\"`"+`
	// Add your response fields here
	// Example: Name string `+"`json:\"name\"`"+`
}

// ========================================
// Mappers
// ========================================

// To%sModel converts a Create%sRequest to a models.%s
func To%sModel(req *Create%sRequest) *models.%s {
	if req == nil {
		return nil
	}
	return &models.%s{
		// Map request fields to model fields
		// Example: Name: req.Name,
	}
}

// To%sResponse converts a models.%s to a %sResponse
func To%sResponse(model *models.%s) %sResponse {
	if model == nil {
		return %sResponse{}
	}
	return %sResponse{
		ID:        model.ID,
		CreatedAt: model.CreatedAt.Format(constants.RFC3339DateTimeFormat),
		// Map model fields to response fields
		// Example: Name: model.Name,
	}
}
`,
		domain,               // 1: package name
		title, domain, title, // 2-4: Create request comment and type
		title, domain, title, // 5-7: Response comment and type
		title, title, title, // 8-10: ToModel comment
		title, title, title, // 11-13: ToModel function signature
		title,               // 14: ToModel return statement
		title, title, title, // 15-17: ToResponse comment
		title, title, title, // 18-20: ToResponse function signature
		title, // 21: ToResponse empty return
		title, // 22: ToResponse populated return
	)
}

func parseText(text string, capitalize bool) string {
	if len(text) == 0 {
		return text
	}

	first := string(text[0])
	rest := text[1:]

	if capitalize {
		return strings.ToUpper(first) + rest
	}
	return strings.ToLower(first) + rest
}
