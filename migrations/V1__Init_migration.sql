CREATE TABLE users
(
    id      uuid,
    name    varchar(255) not null unique,
    balance bigint       not null,
    PRIMARY KEY (id)
);

CREATE TABLE positions
(
    id                 uuid,
    "user"               uuid   not null,
    company            uuid   not null,
    ask_open           bigint not null,
    bid_open           bigint not null,
    is_opened          bool   not null,
    close_profit       bigint not null,
    time_price_open    time   not null,
    count_buy_position bigint,
    max_position_cost  bigint,
    min_position_cost  bigint,
    is_sales           bool   not null,
    is_fixed           bool   not null,
    PRIMARY KEY (id),
    FOREIGN KEY ("user") REFERENCES users (id)
);