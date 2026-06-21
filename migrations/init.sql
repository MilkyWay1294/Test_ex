-- 1. Таблица пользователей
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. Таблица команд
CREATE TABLE IF NOT EXISTS teams (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_by INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    -- Связь 1: teams.created_by -> users.id
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT
);

-- 3. Связь пользователи -> команда (многие-ко-многим) + роль
CREATE TABLE IF NOT EXISTS team_members (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    team_id INT NOT NULL,
    role ENUM('owner', 'admin', 'member') NOT NULL DEFAULT 'member',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_user_team (user_id, team_id),
    -- Связь 2: team_members.user_id -> users.id
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    -- Связь 3: team_members.team_id -> teams.id
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
);

-- 4. Таблица задач
CREATE TABLE IF NOT EXISTS tasks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status ENUM('todo', 'in_progress', 'review', 'done') NOT NULL DEFAULT 'todo',
    team_id INT NOT NULL,
    assignee_id INT,
    created_by INT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    -- Связь 4: tasks.assignee_id -> users.id
    FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE SET NULL,
    -- Связь 5: tasks.team_id -> teams.id
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
    -- Связь 6: tasks.created_by -> users.id
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE RESTRICT
);

-- 5. История изменений задач (аудит)
CREATE TABLE IF NOT EXISTS task_history (
    id INT AUTO_INCREMENT PRIMARY KEY,
    task_id INT NOT NULL,
    changed_by INT NOT NULL,
    field_name VARCHAR(50) NOT NULL, -- что изменилось (status, title, assignee и т.д.)
    old_value TEXT,
    new_value TEXT,
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    -- Связь 7: task_history.task_id -> tasks.id
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    -- Связь 8: task_history.changed_by -> users.id
    FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE RESTRICT
);

-- 6. Комментарии к задачам
CREATE TABLE IF NOT EXISTS task_comments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    task_id INT NOT NULL,
    user_id INT NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    -- Связь 9: task_comments.task_id -> tasks.id
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    -- Связь 10: task_comments.user_id -> users.id
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Индексы для оптимизации будущих сложных SQL-запросов (требование из ТЗ)
CREATE INDEX idx_tasks_team ON tasks(team_id);
CREATE INDEX idx_team_members_user ON team_members(user_id);