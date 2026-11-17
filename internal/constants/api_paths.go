package constants

var PublicAPIPaths = []string{
	"/api/v1/auth",
	"/api/v1/health",
}

var BasicProtectedAPIPaths = []string{
	"/api/v1/profile",
	"/api/v1/resources",
}

var SwaggerAPIPaths = []string{
	"/swagger",
	"/docs",
	"/api-docs",
	"/openapi",
}

func GetAPIPathsToSkip() []string {
	var allSkipPaths []string
	allSkipPaths = append(allSkipPaths, PublicAPIPaths...)
	allSkipPaths = append(allSkipPaths, BasicProtectedAPIPaths...)
	allSkipPaths = append(allSkipPaths, SwaggerAPIPaths...)
	return allSkipPaths
}
