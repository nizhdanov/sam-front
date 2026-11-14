package buildconfig

import "testing"

func TestSealedConfigurationIgnoresRuntimeOverrides(t *testing.T) {
	values, err := environment([]byte(`{"JWT_SERVICE_SECRET":"compiled","HTTP_ADDR":":8081"}`),
		[]string{"JWT_SERVICE_SECRET=runtime", "EXTRA=unexpected"})
	if err != nil {
		t.Fatal(err)
	}
	if values["JWT_SERVICE_SECRET"] != "compiled" || values["EXTRA"] != "" {
		t.Fatal("runtime environment changed the sealed configuration")
	}
}

func TestDevelopmentUsesProcessEnvironment(t *testing.T) {
	values, err := environment([]byte(`{}`), []string{"POSTGRES_DSN=postgres://local?x=a=b"})
	if err != nil || values["POSTGRES_DSN"] != "postgres://local?x=a=b" {
		t.Fatal("development environment was not preserved")
	}
}

func TestInvalidConfigurationDoesNotExposeContent(t *testing.T) {
	_, err := environment([]byte(`{"secret":"do-not-log"`), nil)
	if err == nil || err.Error() != "invalid embedded configuration" {
		t.Fatal("expected a sanitized configuration error")
	}
}
