package handlers

import (
	"avito-user-balance/models"
	"avito-user-balance/repositories/postgres"
	"avito-user-balance/validate"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

type AppHandler struct {
	l  *log.Logger
	hu *UserHandler
	tr *postgres.TransactionsRepository
}

func NewAppHandler(lg *log.Logger, hu *UserHandler, d *sql.DB) *AppHandler {
	return &(AppHandler{lg, hu,
		postgres.NewTransactionsRepository(d)})
}

func (ha *AppHandler) PostTransaction(rw http.ResponseWriter, rq *http.Request) {
	newTr := rq.Context().Value(KeyTransactionPost{}).(*models.Transaction)
	usrUpd := &models.User{newTr.UserId, newTr.Value, newTr.ReserveValue}

	err := ha.hu.updateUserData(usrUpd)
	if err == nil {
		err = ha.tr.AddTransaction(newTr)
	}
	if err != nil {
		SendError(http.StatusInternalServerError, err, rw)
		return
	}
	SendSuccessful(rw)
}

func (ha *AppHandler) PostTransfer(rw http.ResponseWriter, rq *http.Request) {
	tf := rq.Context().Value(KeyTransfer{}).(*models.Transfer)
	senderChanges := &models.User{tf.Sender, -tf.Value, 0}
	recipientChanges := &models.User{tf.Recipient, tf.Value, 0}

	err := ha.hu.updateUserData(senderChanges)
	if err == nil {
		err = ha.hu.updateUserData(recipientChanges)
		if err != nil {
			senderChanges.Balance = recipientChanges.Balance
			err = ha.hu.updateUserData(senderChanges)
		}
	}
	if err != nil {
		SendError(http.StatusInternalServerError, err, rw)
		return
	}
	senderTr, recipTr := transactionsFromTransfer(tf)
	err = ha.tr.AddTransaction(senderTr)
	err = ha.tr.AddTransaction(recipTr)
	SendSuccessful(rw)
}

func transactionsFromTransfer(tf *models.Transfer) (*models.Transaction, *models.Transaction) {
	senderTr := &models.Transaction{0, 0, tf.Sender, 0,
		-tf.Value, 0, time.Now(), "approved", "Transfer to another user"}
	recTr := &models.Transaction{0, 0, tf.Recipient, 0,
		tf.Value, 0, time.Now(), "approved", "Transfer from another user"}
	return senderTr, recTr
}

func (ha *AppHandler) GetTransactions(rw http.ResponseWriter, rq *http.Request) {
	trs, err := ha.tr.FindAllTransactions()
	if err != nil {
		SendError(http.StatusNotFound, err, rw)
		return
	}

	err = transactionsToJSON(trs, rw)

	if err != nil {
		SendJSONError(err, "encoding Transactions", rw)
		return
	}
}

func (ha *AppHandler) GetHistory(rw http.ResponseWriter, rq *http.Request) {
	request := rq.Context().Value(KeyHistory{}).(*models.HistoryRequest)
	history_page := &models.History{}
	var err error

	var sort string
	if request.Sort == "by_date" {
		sort = "time_st"
	} else {
		sort = "ABS(cost)"
	}
	
	history_page.History, err = ha.tr.FindUsersTransaction(
		request.UserId,
		request.Page,
		sort)
	if err != nil {
		SendError(http.StatusNotFound, err, rw)
		return
	}

	err = historyToJSON(history_page, rw)
	if err != nil {
		SendJSONError(err, "encoding Transactions", rw)
		return
	}
}

func (ha *AppHandler) GetTransaction(rw http.ResponseWriter, rq *http.Request) {
	vars := mux.Vars(rq)
	id, err := strconv.Atoi(vars["id"])

	if err != nil {
		SendError(http.StatusBadRequest,
			errors.New("Invalid Transaction Id"),
			rw)
		return
	}

	var tr *models.Transaction
	tr, err = ha.tr.FindLastTransactionByOrder(id)
	if tr == nil {
		SendError(http.StatusNotFound, err, rw)
		return
	}

	err = transactionToJSON(*tr, rw)
	if err != nil {
		SendJSONError(err, "encoding Transfer", rw)
		return
	}
}

func historyFromJSON(hReq *models.HistoryRequest, rd io.Reader) error {
	decoder := json.NewDecoder(rd)
	return decoder.Decode(hReq)
}

func historyToJSON(h *models.History, wr http.ResponseWriter) error {
	wr.Header().Set("Content-type", "application/json")
	encoder := json.NewEncoder(wr)
	return encoder.Encode(h)
}

func transferFromJSON(tf *models.Transfer, rd io.Reader) error {
	decoder := json.NewDecoder(rd)
	return decoder.Decode(tf)
}

func transactionFromJSON(tr *models.Transaction, rd io.Reader) error {
	decoder := json.NewDecoder(rd)
	return decoder.Decode(tr)
}

func transactionsToJSON(trs models.Transactions, wr http.ResponseWriter) error {
	wr.Header().Set("Content-type", "application/json")
	encoder := json.NewEncoder(wr)
	return encoder.Encode(trs)
}

func transactionToJSON(tr models.Transaction, wr http.ResponseWriter) error {
	wr.Header().Set("Content-type", "application/json")
	encoder := json.NewEncoder(wr)
	return encoder.Encode(tr)
}

// validate user prerequisites
func (ha *AppHandler) ValidateUser(tr *models.Transaction, err *error) {
	if *err != nil {
		return
	}
	validate.ValidateUserID(tr.UserId, err)
	userDB := ha.hu.UserInDb(tr.UserId, err)
	if *err != nil {
		return
	}
	// validate User's balance and reserve values for the transaction
	userUpd := &models.User{userDB.ID, tr.Value, tr.ReserveValue}
	validate.ValidateUserData(userUpd, userDB, err)
}

type KeyTransactionPost struct{}

func (ha *AppHandler) PrepareTransactionValue(tr *models.Transaction) {
	switch tr.Status {
	case "", "in process":
		tr.ReserveValue = tr.Value
		tr.Value *= -1
	case "approved":
		tr.ReserveValue = tr.Value * -1
		tr.Value = 0
	case "canceled":
		tr.ReserveValue = tr.Value * -1
	}
}

func (ha *AppHandler) ValidateTransfer(tf *models.Transfer, err *error) {
	sender := ha.hu.UserInDb(tf.Sender, err)
	_ = ha.hu.UserInDb(tf.Recipient, err)
	if *err != nil {
		return
	}
	if sender.Balance < tf.Value {
		*err = models.ErrNotEnoughCredit
	}
}

type KeyTransfer struct{}

type KeyHistory struct{}
