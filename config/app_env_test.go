package config

import "testing"

func TestValidateAutoMigrateAllowed_AllowsDevLikeEnvs(t *testing.T) {
	allowed := []string{"", "dev", "development", "local", "test", "testing", "DEV", "  Local  "}

	for _, env := range allowed {
		env := env
		t.Run(env, func(t *testing.T) {
			if err := ValidateAutoMigrateAllowed(env); err != nil {
				t.Fatalf("expected no error for env %q, got %v", env, err)
			}
		})
	}
}

func TestValidateAutoMigrateAllowed_RejectsProdAndOtherEnvs(t *testing.T) {
	rejected := []string{"prod", "production", "staging", "preprod", " Production ", "qa"}

	for _, env := range rejected {
		env := env
		t.Run(env, func(t *testing.T) {
			if err := ValidateAutoMigrateAllowed(env); err == nil {
				t.Fatalf("expected error for env %q, got nil", env)
			}
		})
	}
}
