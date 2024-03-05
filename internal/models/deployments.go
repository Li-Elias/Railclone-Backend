package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Li-Elias/Railclone/internal/validator"
)

type Deployment struct {
	ID          int64             `json:"id"`
	Image       string            `json:"image"`
	Port        int32             `json:"port"`
	Volume      int32             `json:"volume,omitempty"`
	Replicas    int32             `json:"replicas"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	LastUpdated time.Time         `json:"last_updated"`
	UserID      int64             `json:"-"`
	Running     bool              `json:"running"`
}

type DeploymentModel struct {
	DB *sql.DB
}

type AvailableDeploymentData struct {
	Volume  bool
	EnvVars []string
}

var AvailableDeployments = map[string]AvailableDeploymentData{
	"postgres": {
		Volume: true,
		EnvVars: []string{
			"POSTGRES_DB",
			"POSTGRES_PASSWORD",
			"POSTGRES_USER",
		},
	},
	"redis": {
		Volume:  true,
		EnvVars: []string{},
	},
	"mysql": {
		Volume: true,
		EnvVars: []string{
			"MYSQL_DATABASE",
			"MYSQLPASSWORD",
			"MYSQLUSER",
		},
	},
	"mongo": {
		Volume: true,
		EnvVars: []string{
			"MONGO_DB_NAME",
			"MONGOPASSWORD",
			"MONGOUSER",
		},
	},
}

func ValidateDeployment(v *validator.Validator, deployment *Deployment) {
	v.Check(deployment.Image != "", "image", "must be provided")
	_, exist := AvailableDeployments[deployment.Image]
	v.Check(exist, "image", "needs to be available")
	v.Check(deployment.Volume >= 0, "volume", "cannot have a negative value")
	v.Check(deployment.Volume <= 5, "volume", "cannot have a value over 5")
	v.Check(AvailableDeployments[deployment.Image].Volume && deployment.Volume != 0, "volume", "not available for this image")
	v.Check(deployment.Replicas >= 1, "replicas", "needs to have a value of at least 1")
	v.Check(deployment.Replicas <= 4, "replicas", "cannot have a value over 4")
	v.Check(validator.CheckEnvVars(deployment.EnvVars, AvailableDeployments[deployment.Image].EnvVars), "env_vars", "not available or valid")
}

func (m DeploymentModel) Insert(deployment *Deployment) error {
	query := `
		INSERT INTO deployments (image, port, volume, replicas, env_vars, user_id, running)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, last_updated`

	envVars, err := json.Marshal(deployment.EnvVars)
	if err != nil {
		return err
	}

	args := []interface{}{
		deployment.Image,
		deployment.Port,
		deployment.Volume,
		deployment.Replicas,
		envVars,
		deployment.UserID,
		deployment.Running,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.DB.QueryRowContext(ctx, query, args...).Scan(
		&deployment.ID,
		&deployment.CreatedAt,
		&deployment.LastUpdated,
	)
}

func (m DeploymentModel) GetAllFromUser(userID int64) ([]*Deployment, error) {
	query := `
		SELECT *
		FROM deployments
		WHERE user_id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deployments := []*Deployment{}

	for rows.Next() {
		var envVars []byte
		var deployment Deployment
		err := rows.Scan(
			&deployment.ID,
			&deployment.Image,
			&deployment.Port,
			&deployment.Volume,
			&deployment.Replicas,
			&envVars,
			&deployment.CreatedAt,
			&deployment.LastUpdated,
			&deployment.UserID,
			&deployment.Running,
		)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(envVars, &deployment.EnvVars)
		if err != nil {
			return nil, err
		}

		deployments = append(deployments, &deployment)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return deployments, nil
}

func (m DeploymentModel) GetFromUser(id int64, userID int64) (*Deployment, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT *
		FROM deployments
		WHERE id = $1 AND user_id = $2`

	args := []interface{}{id, userID}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var envVars []byte
	var deployment Deployment

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&deployment.ID,
		&deployment.Image,
		&deployment.Port,
		&deployment.Volume,
		&deployment.Replicas,
		&envVars,
		&deployment.CreatedAt,
		&deployment.LastUpdated,
		&deployment.UserID,
		&deployment.Running,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	err = json.Unmarshal(envVars, &deployment.EnvVars)
	if err != nil {
		return nil, err
	}

	return &deployment, nil
}

func (m DeploymentModel) UpdateFromUser(id int64, userID int64, deployment *Deployment) (*Deployment, error) {
	query := `
		UPDATE deployments
		SET last_updated = $1, port = $4, volume = $5, replicas = $6, env_vars = $7, running = $8
		WHERE id = $2 AND user_id = $3
		RETURNING id, image, port, volume, replicas, created_at, last_updated, user_id, running`

	envVars, err := json.Marshal(deployment.EnvVars)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []interface{}{
		time.Now(),
		id,
		userID,
		deployment.Port,
		deployment.Volume,
		deployment.Replicas,
		envVars,
		deployment.Running,
	}

	var updatedDeployment Deployment

	err = m.DB.QueryRowContext(ctx, query, args...).Scan(
		&updatedDeployment.ID,
		&updatedDeployment.Image,
		&updatedDeployment.Port,
		&updatedDeployment.Volume,
		&updatedDeployment.Replicas,
		&updatedDeployment.CreatedAt,
		&updatedDeployment.LastUpdated,
		&updatedDeployment.UserID,
		&updatedDeployment.Running,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	updatedDeployment.EnvVars = deployment.EnvVars

	return &updatedDeployment, nil
}

func (m DeploymentModel) DeleteFromUser(id int64, userID int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
		DELETE FROM deployments
		WHERE id = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}
