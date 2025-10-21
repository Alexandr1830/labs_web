package repository

import (
	"fmt"
	"strings"
)

type Repository struct {
}

func NewRepository() (*Repository, error) {
	return &Repository{}, nil
}

type Order struct {
	ID          int
	Title       string
	Type        string
	Date        string
	LastEdit    string
	Description string
	Author      []string
}

type Application struct {
	ID        int
	Documents []Document
}

type Document struct {
	Title   string
	Authors []string
	Emails  []string
	Status  string // "open" или "closed"
}

func (r *Repository) GetOrders() ([]Order, error) {
	orders := []Order{
		{
			ID: 1,
			Title: "Техническое задание на модуль авторизации",
			Type: "DOC",
			Date: "12 сент. 2025",
			LastEdit: "20 сент. 2025",
			Description: "Документ описывает структуру, требования и функционал модуля авторизации пользователей.",
			Author: []string{"Найденко А.В.", "Иванов И.И.", "Петров П.П.", "Сидоров И.П.", "Петряков С.В."},
		},
		{
			ID: 2,
			Title: "API спецификация сервиса",
			Type: "DOC",
			Date: "12 авг. 2025",
			LastEdit: "15 авг. 2025",
			Description: "Спецификация REST API для взаимодействия микросервисов.",
			Author: []string{"Иванов И.И.", "Петров П.П.", "Сидоров И.П."},
		},
		{
			ID: 3,
			Title: "Отчёт по нагрузочному тестированию",
			Type: "XLS",
			Date: "5 авг. 2025",
			LastEdit: "8 авг. 2025",
			Description: "Отчёт о проведённом нагрузочном тестировании сервисов и анализ производительности.",
			Author: []string{"Иванов И.И.", "Петров П.П."},
		},
		{
			ID: 4,
			Title: "Матрица распределения задач",
			Type: "XLS",
			Date: "17 февр. 2025",
			LastEdit: "20 февр. 2025",
			Description: "Матрица распределения задач между членами команды по проекту.",
			Author: []string{"Иванов И.И.", "Петров П.П."},
		},
		{
			ID: 5,
			Title: "Тестовые данные для базы данных",
			Type: "SQL",
			Date: "23 окт. 2024",
			LastEdit: "25 окт. 2024",
			Description: "SQL-скрипты и примеры тестовых данных для наполнения базы данных.",
			Author: []string{"Иванов И.И.", "Петров П.П."},
		},
	}

	if len(orders) == 0 {
		return nil, fmt.Errorf("массив пустой")
	}

	return orders, nil
}

func (r *Repository) GetOrder(id int) (Order, error) {
	// тут у вас будет логика получения нужной услуги, тоже наверное через цикл в первой лабе, и через запрос к БД начиная со второй
	orders, err := r.GetOrders()
	if err != nil {
		return Order{}, err // тут у нас уже есть кастомная ошибка из нашего метода, поэтому мы можем просто вернуть ее
	}

	for _, order := range orders {
		if order.ID == id {
			return order, nil // если нашли, то просто возвращаем найденный заказ (услугу) без ошибок
		}
	}
	return Order{}, fmt.Errorf("заказ не найден") // тут нужна кастомная ошибка, чтобы понимать на каком этапе возникла ошибка и что произошло
}

func (r *Repository) GetOrdersByTitle(title string) ([]Order, error) {
	orders, err := r.GetOrders()
	if err != nil {
		return []Order{}, err
	}

	var result []Order
	for _, order := range orders {
		if strings.Contains(strings.ToLower(order.Title), strings.ToLower(title)) {
			result = append(result, order)
		}
	}

	return result, nil
}

func (r *Repository) GetApplication(id int) (Application, error) {
	applications := []Application{
		{
			ID: 1,
			Documents: []Document{
				{
					Title:   "Техническое задание на модуль авторизации",
					Authors: []string{"Найденко А.В.", "Иванов И.И."},
					Emails:  []string{"naidenko@mail.ru", "ivanov@mail.ru"},
					Status:  "open",
				},
				{
					Title:   "API спецификация сервиса",
					Authors: []string{"Петров П.П."},
					Emails:  []string{"petrov@mail.ru"},
					Status:  "closed",
				},
			},
		},
		{
			ID: 2,
			Documents: []Document{
				{
					Title:   "Отчёт по нагрузочному тестированию",
					Authors: []string{"Иванов И.И.", "Петров П.П."},
					Emails:  []string{"ivanov@mail.ru", "petrov@mail.ru"},
					Status:  "closed",
				},
				{
					Title:   "Матрица распределения задач",
					Authors: []string{"Сидоров С.С."},
					Emails:  []string{"sidorov@mail.ru"},
					Status:  "open",
				},
			},
		},
	}

	// Ищем заявку по ID
	for _, app := range applications {
		if app.ID == id {
			return app, nil
		}
	}

	return Application{}, fmt.Errorf("заявка с ID %d не найдена", id)
}


