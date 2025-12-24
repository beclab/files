-- change db
\connect os_framework_ccnet

-- patch emailuser table
ALTER TABLE public.emailuser ADD COLUMN IF NOT EXISTS is_department_owner smallint NOT NULL DEFAULT 0;

ALTER TABLE public.emailuser ALTER COLUMN is_department_owner SET DEFAULT 0;

UPDATE public.emailuser SET is_department_owner = 0 WHERE is_department_owner IS NULL;

ALTER TABLE public.emailuser ALTER COLUMN is_department_owner SET NOT NULL;

CREATE INDEX IF NOT EXISTS emailuser_is_active_idx ON public.emailuser (is_active);

CREATE INDEX IF NOT EXISTS emailuser_is_department_owner_idx ON public.emailuser (is_department_owner);

ALTER TABLE public.emailuser OWNER TO seafile_os_framework;
ALTER INDEX public.emailuser_is_active_idx OWNER TO seafile_os_framework;
ALTER INDEX public.emailuser_is_department_owner_idx OWNER TO seafile_os_framework;


--
-- Name: OrgFileExtWhiteList; Type: TABLE; Schema: public; Owner: seafile_os_framework
--
CREATE SEQUENCE IF NOT EXISTS public.orgfileextwhitelist_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.OrgFileExtWhiteList (
    id BIGINT NOT NULL DEFAULT nextval('public.orgfileextwhitelist_id_seq'),
    org_id INTEGER,
    white_list TEXT
    );

ALTER SEQUENCE public.orgfileextwhitelist_id_seq OWNED BY public.OrgFileExtWhiteList.id;

ALTER TABLE public.OrgFileExtWhiteList OWNER TO seafile_os_framework;
ALTER SEQUENCE public.orgfileextwhitelist_id_seq OWNER TO seafile_os_framework;

CREATE UNIQUE INDEX IF NOT EXISTS orgfileextwhitelist_org_id_unique ON public.OrgFileExtWhiteList (org_id);

ALTER INDEX public.orgfileextwhitelist_org_id_unique OWNER TO seafile_os_framework;

-- change db
\connect os_framework_seafile

--
-- Name: GCID; Type: TABLE; Schema: public; Owner: seafile_os_framework
--

CREATE SEQUENCE IF NOT EXISTS public.gcid_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.GCID (
    id BIGINT NOT NULL DEFAULT nextval('public.gcid_id_seq'),
    repo_id CHAR(36),
    gc_id CHAR(36),
    CONSTRAINT gcid_repo_id_unique UNIQUE (repo_id)
    );

ALTER SEQUENCE public.gcid_id_seq OWNED BY public.GCID.id;

ALTER TABLE public.GCID OWNER TO seafile_os_framework;
ALTER SEQUENCE public.gcid_id_seq OWNER TO seafile_os_framework;

--
-- Name: LastGCID; Type: TABLE; Schema: public; Owner: seafile_os_framework
--

CREATE SEQUENCE IF NOT EXISTS public.lastgcid_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.LastGCID (
    id BIGINT NOT NULL DEFAULT nextval('public.lastgcid_id_seq'),
    repo_id CHAR(36),
    client_id VARCHAR(128),
    gc_id CHAR(36),
    CONSTRAINT lastgcid_repo_client_unique UNIQUE (repo_id, client_id)
    );

ALTER SEQUENCE public.lastgcid_id_seq OWNED BY public.LastGCID.id;

ALTER TABLE public.LastGCID OWNER TO seafile_os_framework;
ALTER SEQUENCE public.lastgcid_id_seq OWNER TO seafile_os_framework;

--
-- Name: webuploadtempfiles_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX IF NOT EXISTS webuploadtempfiles_repoid_idx ON public.webuploadtempfiles USING btree (repo_id);
ALTER INDEX public.webuploadtempfiles_repoid_idx OWNER TO seafile_os_framework;

--
-- Name: RoleUploadRateLimit; Type: TABLE; Schema: public; Owner: seafile_os_framework
--
CREATE SEQUENCE IF NOT EXISTS public.roleuploadratelimit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.RoleUploadRateLimit (
    id BIGINT NOT NULL DEFAULT nextval('public.roleuploadratelimit_id_seq'),
    role VARCHAR(255),
    upload_limit BIGINT,
    CONSTRAINT roleuploadratelimit_role_unique UNIQUE (role)
    );

ALTER SEQUENCE public.roleuploadratelimit_id_seq OWNED BY public.RoleUploadRateLimit.id;
ALTER TABLE public.RoleUploadRateLimit OWNER TO seafile_os_framework;
ALTER SEQUENCE public.roleuploadratelimit_id_seq OWNER TO seafile_os_framework;
ALTER INDEX public.roleuploadratelimit_role_unique OWNER TO seafile_os_framework;

--
-- Name: RoleDownloadRateLimit; Type: TABLE; Schema: public; Owner: seafile_os_framework
--
CREATE SEQUENCE IF NOT EXISTS public.roledownloadratelimit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.RoleDownloadRateLimit (
    id BIGINT NOT NULL DEFAULT nextval('public.roledownloadratelimit_id_seq'),
    role VARCHAR(255),
    download_limit BIGINT,
    CONSTRAINT roledownloadratelimit_role_unique UNIQUE (role)
    );

ALTER SEQUENCE public.roledownloadratelimit_id_seq OWNED BY public.RoleDownloadRateLimit.id;
ALTER TABLE public.RoleDownloadRateLimit OWNER TO seafile_os_framework;
ALTER SEQUENCE public.roledownloadratelimit_id_seq OWNER TO seafile_os_framework;
ALTER INDEX public.roledownloadratelimit_role_unique OWNER TO seafile_os_framework;

--
-- Name: UserUploadRateLimit; Type: TABLE; Schema: public; Owner: seafile_os_framework
--
CREATE SEQUENCE IF NOT EXISTS public.useruploadratelimit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.UserUploadRateLimit (
    id BIGINT NOT NULL DEFAULT nextval('public.useruploadratelimit_id_seq'),
    "user" VARCHAR(255),
    upload_limit BIGINT,
    CONSTRAINT useruploadratelimit_user_unique UNIQUE ("user")
    );

ALTER SEQUENCE public.useruploadratelimit_id_seq OWNED BY public.UserUploadRateLimit.id;
ALTER TABLE public.UserUploadRateLimit OWNER TO seafile_os_framework;
ALTER SEQUENCE public.useruploadratelimit_id_seq OWNER TO seafile_os_framework;
ALTER INDEX public.useruploadratelimit_user_unique OWNER TO seafile_os_framework;

--
-- Name: UserDownloadRateLimit; Type: TABLE; Schema: public; Owner: seafile_os_framework
--
CREATE SEQUENCE IF NOT EXISTS public.userdownloadratelimit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.UserDownloadRateLimit (
    id BIGINT NOT NULL DEFAULT nextval('public.userdownloadratelimit_id_seq'),
    "user" VARCHAR(255),
    download_limit BIGINT,
    CONSTRAINT userdownloadratelimit_user_unique UNIQUE ("user")
    );

ALTER SEQUENCE public.userdownloadratelimit_id_seq OWNED BY public.UserDownloadRateLimit.id;
ALTER TABLE public.UserDownloadRateLimit OWNER TO seafile_os_framework;
ALTER SEQUENCE public.userdownloadratelimit_id_seq OWNER TO seafile_os_framework;
ALTER INDEX public.userdownloadratelimit_user_unique OWNER TO seafile_os_framework;

--
-- Name: OrgUserDefaultQuota; Type: TABLE; Schema: public; Owner: seafile_os_framework
--
CREATE SEQUENCE IF NOT EXISTS public.orguserdefaultquota_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.OrgUserDefaultQuota (
    id BIGINT NOT NULL DEFAULT nextval('public.orguserdefaultquota_id_seq'),
    org_id INTEGER,
    quota BIGINT,
    CONSTRAINT orguserdefaultquota_org_id_unique UNIQUE (org_id)
    );

ALTER SEQUENCE public.orguserdefaultquota_id_seq OWNED BY public.OrgUserDefaultQuota.id;
ALTER TABLE public.OrgUserDefaultQuota OWNER TO seafile_os_framework;
ALTER SEQUENCE public.orguserdefaultquota_id_seq OWNER TO seafile_os_framework;
ALTER INDEX public.orguserdefaultquota_org_id_unique OWNER TO seafile_os_framework;

--
-- Name: OrgDownloadRateLimit; Type: TABLE; Schema: public; Owner: seafile_os_framework
--
CREATE SEQUENCE IF NOT EXISTS public.orgdownloadratelimit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.OrgDownloadRateLimit (
    id BIGINT NOT NULL DEFAULT nextval('public.orgdownloadratelimit_id_seq'),
    org_id INTEGER,
    download_limit BIGINT,
    CONSTRAINT orgdownloadratelimit_org_id_unique UNIQUE (org_id)
    );

ALTER SEQUENCE public.orgdownloadratelimit_id_seq OWNED BY public.OrgDownloadRateLimit.id;
ALTER TABLE public.OrgDownloadRateLimit OWNER TO seafile_os_framework;
ALTER SEQUENCE public.orgdownloadratelimit_id_seq OWNER TO seafile_os_framework;
ALTER INDEX public.orgdownloadratelimit_org_id_unique OWNER TO seafile_os_framework;

--
-- Name: OrgUploadRateLimit; Type: TABLE; Schema: public; Owner: seafile_os_framework
--
CREATE SEQUENCE IF NOT EXISTS public.orguploadratelimit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

CREATE TABLE IF NOT EXISTS public.OrgUploadRateLimit (
    id BIGINT NOT NULL DEFAULT nextval('public.orguploadratelimit_id_seq'),
    org_id INTEGER,
    upload_limit BIGINT,
    CONSTRAINT orguploadratelimit_org_id_unique UNIQUE (org_id)
    );

ALTER SEQUENCE public.orguploadratelimit_id_seq OWNED BY public.OrgUploadRateLimit.id;
ALTER TABLE public.OrgUploadRateLimit OWNER TO seafile_os_framework;
ALTER SEQUENCE public.orguploadratelimit_id_seq OWNER TO seafile_os_framework;
ALTER INDEX public.orguploadratelimit_org_id_unique OWNER TO seafile_os_framework;
