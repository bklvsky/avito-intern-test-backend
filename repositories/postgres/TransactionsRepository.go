package postgres

import (
	"avito-user-balance/models"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type TransactionsRepository struct {
	db *sql.DB
}

func NewTransactionsRepository(d *sql.DB) *TransactionsRepository {
	return &(TransactionsRepository{d})
}

func (tr *TransactionsRepository) FindLastTransactionByOrder(id int) (*models.Transaction, error) {
	t := models.Transaction{}

	queryString := ("SELECT * FROM transactions " +
		"WHERE order_id=$1 " +
		"ORDER BY time_st DESC LIMIT 1;")

	row := tr.db.QueryRow(queryString, id)
	err := row.Scan(&t.ID, &t.OrderId, &t.UserId, &t.ServiceId,
		&t.Value, &t.Timesp, &t.Note, &t.Status)

	switch err {
	case sql.ErrNoRows:
		return nil, fmt.Errorf("No Transaction with OrderID %d found", id) // 404 Not found with err struct
	case nil:
		return &t, nil
	default:
		return nil, err
	}
}

func (tr *TransactionsRepository) FindAllTransactions() (models.Transactions, error) {
	var trs models.Transactions

	rows, err := tr.db.Query("SELECT * FROM transactions;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t = models.Transaction{}
		err := rows.Scan(&t.ID, &t.OrderId, &t.UserId, &t.ServiceId,
			&t.Value, &t.Timesp, &t.Note, &t.Status)
		if err != nil {
			return nil, err
		}
		trs = append(trs, &t)
	}
	return trs, nil
}

func (tr *TransactionsRepository) AddTransaction(t *models.Transaction) error {
	queryStr := ("INSERT INTO transactions " +
		"(order_id, user_id, service_id, cost, time_st, note, status) " +
		"VALUES ($1, $2, $3, $4, $5, $6, $7);")

	_, err := tr.db.Exec(queryStr,
		t.OrderId, t.UserId, t.ServiceId, t.Value, t.Timesp, t.Note, t.Status)
	return err
}
