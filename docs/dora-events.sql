create table if not exists dora_deployment_events (
  id bigserial primary key,
  service text not null,
  environment text not null,
  commit_sha text not null,
  pipeline_id text not null,
  deployed_at timestamptz not null,
  commit_created_at timestamptz,
  incident_id text,
  rolled_back boolean not null default false,
  created_at timestamptz not null default now()
);

create index if not exists dora_deployment_events_service_env_time_idx
  on dora_deployment_events (service, environment, deployed_at desc);

create or replace view dora_lead_time as
select
  service,
  environment,
  deployed_at::date as deployment_date,
  percentile_cont(0.5) within group (
    order by extract(epoch from (deployed_at - commit_created_at)) / 60
  ) as median_lead_time_minutes
from dora_deployment_events
where commit_created_at is not null
group by service, environment, deployed_at::date;

