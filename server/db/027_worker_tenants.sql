-- Background workers have no authenticated request from which to obtain a
-- tenant. Expose only tenant identifiers; all work remains subject to RLS.
CREATE OR REPLACE FUNCTION litterbox_worker_tenants()
RETURNS TABLE (id uuid)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public AS $$
    SELECT t.id FROM public.tenants AS t ORDER BY t.id;
$$;
REVOKE ALL ON FUNCTION litterbox_worker_tenants() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION litterbox_worker_tenants() TO litterbox_app;
