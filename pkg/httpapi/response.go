package httpapi

import "feedback/pkg/app"

type publicSessionUser struct {
	Matricula    string `json:"matricula"`
	Name         string `json:"name"`
	SchemaStatus string `json:"schemaStatus,omitempty"`
}

func publicUser(user app.SessionUser) publicSessionUser {
	return publicSessionUser{
		Matricula:    user.Matricula,
		Name:         user.Name,
		SchemaStatus: user.SchemaStatus,
	}
}

func publicGradeResults(results app.GradeResults) app.GradeResults {
	output := make(app.GradeResults, len(results))
	for key, result := range results {
		output[key] = publicGradeResult(result)
	}
	return output
}

func publicGradeResult(result app.GradeResult) app.GradeResult {
	result.SpreadsheetID = ""
	for tableIdx := range result.Tables {
		result.Tables[tableIdx].SpreadsheetID = ""
	}
	return result
}
