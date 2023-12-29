package main

import (
	"context"
	"go-todo-app/page"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}

	server := &http.Server{Addr: port}
	connStr := os.Getenv("CONNSTR")

	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Loading Index Page...\n")
		rows, _ := conn.Query(context.Background(), "SELECT * FROM task_list")
		taskItems, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (page.TaskItem, error) {
			var id int
			var task string
			err := row.Scan(&id, &task)
			return page.TaskItem{Id: id, Task: task}, err

		})
		if err != nil {
			log.Fatal(err)
		}
		err = page.Index(taskItems).Render(r.Context(), w)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Index Page Loaded\n")
	})

	http.HandleFunc("/page/edit/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Handling edit path\n")
		segments := strings.Split(r.URL.Path, "/")
		id, err := strconv.Atoi(segments[len(segments)-1])
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Path to be edited: %d\n", id)
		rows, _ := conn.Query(context.Background(), "SELECT * FROM task_list WHERE id = $1", id)
		taskItems, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (page.TaskItem, error) {
			var id int
			var task string
			err := row.Scan(&id, &task)
			return page.TaskItem{Id: id, Task: task}, err

		})
		if err != nil {
			log.Fatal(err)
		}

		err = page.Form(taskItems[0]).Render(r.Context(), w)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Task successfully obtained to be edited\n")
	})

	http.HandleFunc("/page/taskItem/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Handling taskItem path\n")
		segments := strings.Split(r.URL.Path, "/")
		id, err := strconv.Atoi(segments[len(segments)-1])
		if err != nil {
			log.Fatal(err)
		}
		if r.Method == http.MethodGet {
			log.Printf("Get Method attempt for task %d\n", id)
			rows, _ := conn.Query(context.Background(), "SELECT * FROM task_list WHERE id = $1", id)
			taskItems, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (page.TaskItem, error) {
				var id int
				var task string
				err := row.Scan(&id, &task)
				return page.TaskItem{Id: id, Task: task}, err

			})
			if err != nil {
				log.Fatal(err)
			}

			err = page.Row(taskItems[0]).Render(r.Context(), w)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Task successfully got\n")
			return
		}
		if r.Method == http.MethodPut {
			log.Printf("Put Method attempt for task %d\n", id)
			r.ParseForm()
			task := r.FormValue("task")
			commandTag, err := conn.Exec(context.Background(), "UPDATE task_list SET task = $1 WHERE id = $2", task, id)
			if err != nil {
				log.Fatal(err)
			}
			if commandTag.RowsAffected() != 1 {
				log.Printf("No Rows Affected")
			}
			err = page.Row(page.TaskItem{Id: id, Task: task}).Render(r.Context(), w)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Task successfully updated\n")
			return
		}
		if r.Method == http.MethodDelete {
			log.Printf("Delete Method attempt for task %d\n", id)
			_, err := conn.Exec(context.Background(), "DELETE FROM task_list WHERE id = $1", id)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Task successfully deleted\n")

			return
		}
		w.WriteHeader(404)

	})
	http.HandleFunc("/page/newTaskItem/form", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("getting new task form\n")
		err = page.NewTaskForm().Render(r.Context(), w)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("successfully obtained form for new task\n")
	})
	http.HandleFunc("/page/newTaskItem/attemptAdd", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("attemping to add new task\n")
		if r.Method == http.MethodPost {
			r.ParseForm()
			task := r.FormValue("newTask")
			log.Printf("%s\n", task)
			var newId int
			err := conn.QueryRow(context.Background(), "INSERT INTO task_list(task) VALUES ($1) RETURNING id", task).Scan(&newId)
			if err != nil {
				log.Fatal(err)
			}
			page.Row(page.TaskItem{Id: newId, Task: task}).Render(r.Context(), w)
			log.Printf("new task successfully added\n")
		}
		log.Printf("Replacing form with button\n")
		page.NewTaskButton().Render(r.Context(), w)
		log.Printf("Form Replaced\n")
	})

	go func() {
		log.Printf("Starting server at port %s...\n", port)
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Printf("Stopping server...\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	log.Printf("%v\n", err)
	os.Exit(0)
}

func addTasks(conn *pgx.Conn, tasks []string) {
	copyCount, err := conn.CopyFrom(
		context.Background(),
		pgx.Identifier{"task_list"},
		[]string{"task"},
		pgx.CopyFromSlice(len(tasks), func(i int) ([]any, error) {
			return []any{tasks[i]}, nil
		}),
	)
	if err != nil {
		log.Fatalf("copyCount: %v, err: %v\n", copyCount, err)
	}
}
