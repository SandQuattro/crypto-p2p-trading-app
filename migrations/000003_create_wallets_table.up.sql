create table if not exists wallets
(
    id              bigserial primary key,
    user_id         bigint       not null,
    address         varchar(255) not null unique,
    derivation_path varchar(255) not null,
    created_at      timestamp with time zone default CURRENT_TIMESTAMP,
    wallet_index    integer                  default 0,

    constraint unique_user_wallet_index
        unique (user_id, wallet_index)
);

create index idx_wallets_address
    on public.wallets (address);

create index idx_wallets_index
    on public.wallets (wallet_index);

create index idx_wallets_user_id
    on public.wallets (user_id);

