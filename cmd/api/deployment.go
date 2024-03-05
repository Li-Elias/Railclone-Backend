package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Li-Elias/Railclone/internal/deployments"
	"github.com/Li-Elias/Railclone/internal/models"
	"github.com/Li-Elias/Railclone/internal/validator"
	"github.com/go-chi/chi/v5"
)

func (app *application) listAvailableDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	number := 0
	for range models.AvailableDeployments {
		number++
	}

	env := envelope{
		"number":    number,
		"available": models.AvailableDeployments,
	}

	err := app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) createDeploymentHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Image    string            `json:"image"`
		Volume   int32             `json:"volume"`
		Replicas int32             `json:"replicas"`
		EnvVars  map[string]string `json:"env_vars"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	deployment := &models.Deployment{
		Image:    input.Image,
		Volume:   input.Volume,
		Replicas: input.Replicas,
		EnvVars:  input.EnvVars,
		UserID:   user.ID,
		Running:  true,
	}

	v := validator.New()
	if models.ValidateDeployment(v, deployment); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Deployments.Insert(deployment)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	deployment.Port, err = deployments.Create(app.clientset, deployment)
	if err != nil {
		// delete postgres entry because object does not exist anymore
		app.models.Deployments.DeleteFromUser(deployment.ID, user.ID)
		app.serverErrorResponse(w, r, err)
		return
	}

	// Insert port
	deployment, err = app.models.Deployments.UpdateFromUser(deployment.ID, user.ID, deployment)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"deployment": deployment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) getUserDeploymentsHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	deployments, err := app.models.Deployments.GetAllFromUser(user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"deployments": deployments}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) getUserDeploymentHandler(w http.ResponseWriter, r *http.Request) {
	id_str := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(id_str, 10, 64)
	if err != nil || id < 1 {
		app.notFoundResponse(w, r)
		return
	}

	user := app.contextGetUser(r)

	deployment, err := app.models.Deployments.GetFromUser(id, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"deployment": deployment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateUserDeploymentHandler(w http.ResponseWriter, r *http.Request) {
	id_str := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(id_str, 10, 64)
	if err != nil || id < 1 {
		app.notFoundResponse(w, r)
		return
	}

	var input struct {
		Port     int32             `json:"port"`
		Volume   int32             `json:"volume"`
		Replicas int32             `json:"replicas"`
		EnvVars  map[string]string `json:"env_vars"`
		Running  bool              `json:"running"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	updatedDeployment := &models.Deployment{
		ID:       id,
		Port:     input.Port,
		Volume:   input.Volume,
		Replicas: input.Replicas,
		EnvVars:  input.EnvVars,
		UserID:   user.ID,
		Running:  input.Running,
	}

	deployment, err := app.models.Deployments.GetFromUser(id, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	updatedDeployment.Image = deployment.Image

	v := validator.New()
	models.ValidateDeployment(v, updatedDeployment)
	if updatedDeployment.Port != 0 {
		v.Check(updatedDeployment.Port >= 30000, "port", "cannot have a value under 30000")
		v.Check(updatedDeployment.Port <= 32767, "port", "cannot have a value over 32767")
	}
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = deployments.Update(app.clientset, deployment, updatedDeployment)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

	updatedDeployment, err = app.models.Deployments.UpdateFromUser(id, user.ID, updatedDeployment)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusAccepted, envelope{"deployment": updatedDeployment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteUserDeploymentHandler(w http.ResponseWriter, r *http.Request) {
	id_str := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(id_str, 10, 64)
	if err != nil || id < 1 {
		app.notFoundResponse(w, r)
		return
	}

	user := app.contextGetUser(r)

	err = app.models.Deployments.DeleteFromUser(id, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = deployments.Delete(app.clientset, id, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"message": "deployment successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
