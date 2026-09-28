package queries

var (
	GetCustomerByIDQuery = `
	SELECT
		id,
		email,
		name,
		COALESCE(country, '') AS country,
		created_at
	FROM
		customers
	WHERE 
		id = $1
	`

	GetCustomerByEmailQuery = `
	SELECT
		id,
		email,
		name,
		COALESCE(country, '') AS country,
		created_at
	FROM
		customers
	WHERE 
		LOWER(email) = LOWER($1)
	`
)
