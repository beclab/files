--
-- PostgreSQL database dump
--

-- Dumped from database version 14.9 (Ubuntu 14.9-0ubuntu0.22.04.1)
-- Dumped by pg_dump version 14.9 (Ubuntu 14.9-0ubuntu0.22.04.1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: seafile; Type: DATABASE; Schema: -; Owner: postgres
--

-- CREATE DATABASE seafile WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE = 'C.UTF-8';
--
--
-- ALTER DATABASE seafile OWNER TO postgres;

\connect os_framework_seafile

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: branch; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.branch (
    id bigint NOT NULL,
    name character varying(10),
    repo_id character(41),
    commit_id character(41)
);


ALTER TABLE public.branch OWNER TO seafile_os_framework;

--
-- Name: branch_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.branch_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.branch_id_seq OWNER TO seafile_os_framework;

--
-- Name: branch_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.branch_id_seq OWNED BY public.branch.id;


--
-- Name: garbagerepos; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.garbagerepos (
    id bigint NOT NULL,
    repo_id character(36)
);


ALTER TABLE public.garbagerepos OWNER TO seafile_os_framework;

--
-- Name: garbagerepos_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.garbagerepos_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.garbagerepos_id_seq OWNER TO seafile_os_framework;

--
-- Name: garbagerepos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.garbagerepos_id_seq OWNED BY public.garbagerepos.id;


--
-- Name: innerpubrepo; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.innerpubrepo (
    id bigint NOT NULL,
    repo_id character(37),
    permission character(15)
);


ALTER TABLE public.innerpubrepo OWNER TO seafile_os_framework;

--
-- Name: innerpubrepo_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.innerpubrepo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.innerpubrepo_id_seq OWNER TO seafile_os_framework;

--
-- Name: innerpubrepo_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.innerpubrepo_id_seq OWNED BY public.innerpubrepo.id;


--
-- Name: orgquota; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.orgquota (
    id bigint NOT NULL,
    org_id integer,
    quota bigint
);


ALTER TABLE public.orgquota OWNER TO seafile_os_framework;

--
-- Name: orgquota_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.orgquota_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.orgquota_id_seq OWNER TO seafile_os_framework;

--
-- Name: orgquota_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.orgquota_id_seq OWNED BY public.orgquota.id;


--
-- Name: orguserquota; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.orguserquota (
    id bigint NOT NULL,
    org_id integer,
    "user" character varying(255),
    quota bigint
);


ALTER TABLE public.orguserquota OWNER TO seafile_os_framework;

--
-- Name: orguserquota_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.orguserquota_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.orguserquota_id_seq OWNER TO seafile_os_framework;

--
-- Name: orguserquota_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.orguserquota_id_seq OWNED BY public.orguserquota.id;


--
-- Name: repo; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repo (
    id bigint NOT NULL,
    repo_id character(37)
);


ALTER TABLE public.repo OWNER TO seafile_os_framework;

--
-- Name: repo_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repo_id_seq OWNER TO seafile_os_framework;

--
-- Name: repo_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repo_id_seq OWNED BY public.repo.id;


--
-- Name: repofilecount; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repofilecount (
    id bigint NOT NULL,
    repo_id character(36),
    file_count bigint
);


ALTER TABLE public.repofilecount OWNER TO seafile_os_framework;

--
-- Name: repofilecount_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repofilecount_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repofilecount_id_seq OWNER TO seafile_os_framework;

--
-- Name: repofilecount_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repofilecount_id_seq OWNED BY public.repofilecount.id;


--
-- Name: repogroup; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repogroup (
    id bigint NOT NULL,
    repo_id character(37),
    group_id integer,
    user_name character varying(255),
    permission character(15)
);


ALTER TABLE public.repogroup OWNER TO seafile_os_framework;

--
-- Name: repogroup_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repogroup_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repogroup_id_seq OWNER TO seafile_os_framework;

--
-- Name: repogroup_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repogroup_id_seq OWNED BY public.repogroup.id;


--
-- Name: repohead; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repohead (
    id bigint NOT NULL,
    repo_id character(37),
    branch_name character varying(10)
);


ALTER TABLE public.repohead OWNER TO seafile_os_framework;

--
-- Name: repohead_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repohead_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repohead_id_seq OWNER TO seafile_os_framework;

--
-- Name: repohead_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repohead_id_seq OWNED BY public.repohead.id;


--
-- Name: repohistorylimit; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repohistorylimit (
    id bigint NOT NULL,
    repo_id character(37),
    days integer
);


ALTER TABLE public.repohistorylimit OWNER TO seafile_os_framework;

--
-- Name: repohistorylimit_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repohistorylimit_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repohistorylimit_id_seq OWNER TO seafile_os_framework;

--
-- Name: repohistorylimit_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repohistorylimit_id_seq OWNED BY public.repohistorylimit.id;


--
-- Name: repoinfo; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repoinfo (
    id bigint NOT NULL,
    repo_id character(36),
    name character varying(255) NOT NULL,
    update_time bigint,
    version integer,
    is_encrypted integer,
    last_modifier character varying(255),
    status integer DEFAULT 0
);


ALTER TABLE public.repoinfo OWNER TO seafile_os_framework;

--
-- Name: repoinfo_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repoinfo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repoinfo_id_seq OWNER TO seafile_os_framework;

--
-- Name: repoinfo_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repoinfo_id_seq OWNED BY public.repoinfo.id;


--
-- Name: repoowner; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repoowner (
    id bigint NOT NULL,
    repo_id character(37),
    owner_id character varying(255)
);


ALTER TABLE public.repoowner OWNER TO seafile_os_framework;

--
-- Name: repoowner_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repoowner_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repoowner_id_seq OWNER TO seafile_os_framework;

--
-- Name: repoowner_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repoowner_id_seq OWNED BY public.repoowner.id;


--
-- Name: reposize; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.reposize (
    id bigint NOT NULL,
    repo_id character(37),
    size bigint,
    head_id character(41)
);


ALTER TABLE public.reposize OWNER TO seafile_os_framework;

--
-- Name: reposize_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.reposize_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.reposize_id_seq OWNER TO seafile_os_framework;

--
-- Name: reposize_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.reposize_id_seq OWNED BY public.reposize.id;


--
-- Name: repotokenpeerinfo; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repotokenpeerinfo (
    id bigint NOT NULL,
    token character(41),
    peer_id character(41),
    peer_ip character varying(41),
    peer_name character varying(255),
    sync_time bigint,
    client_ver character varying(20)
);


ALTER TABLE public.repotokenpeerinfo OWNER TO seafile_os_framework;

--
-- Name: repotokenpeerinfo_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repotokenpeerinfo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repotokenpeerinfo_id_seq OWNER TO seafile_os_framework;

--
-- Name: repotokenpeerinfo_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repotokenpeerinfo_id_seq OWNED BY public.repotokenpeerinfo.id;


--
-- Name: repotrash; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repotrash (
    id bigint NOT NULL,
    repo_id character(36),
    repo_name character varying(255),
    head_id character(40),
    owner_id character varying(255),
    size bigint,
    org_id integer,
    del_time bigint
);


ALTER TABLE public.repotrash OWNER TO seafile_os_framework;

--
-- Name: repotrash_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repotrash_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repotrash_id_seq OWNER TO seafile_os_framework;

--
-- Name: repotrash_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repotrash_id_seq OWNED BY public.repotrash.id;


--
-- Name: repousertoken; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repousertoken (
    id bigint NOT NULL,
    repo_id character(37),
    email character varying(255),
    token character(41)
);


ALTER TABLE public.repousertoken OWNER TO seafile_os_framework;

--
-- Name: repousertoken_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repousertoken_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repousertoken_id_seq OWNER TO seafile_os_framework;

--
-- Name: repousertoken_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repousertoken_id_seq OWNED BY public.repousertoken.id;


--
-- Name: repovalidsince; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.repovalidsince (
    id bigint NOT NULL,
    repo_id character(37),
    "timestamp" bigint
);


ALTER TABLE public.repovalidsince OWNER TO seafile_os_framework;

--
-- Name: repovalidsince_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.repovalidsince_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.repovalidsince_id_seq OWNER TO seafile_os_framework;

--
-- Name: repovalidsince_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.repovalidsince_id_seq OWNED BY public.repovalidsince.id;


--
-- Name: seafileconf; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.seafileconf (
    id bigint NOT NULL,
    cfg_group character varying(255) NOT NULL,
    cfg_key character varying(255) NOT NULL,
    value character varying(255),
    property integer
);


ALTER TABLE public.seafileconf OWNER TO seafile_os_framework;

--
-- Name: seafileconf_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.seafileconf_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.seafileconf_id_seq OWNER TO seafile_os_framework;

--
-- Name: seafileconf_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.seafileconf_id_seq OWNED BY public.seafileconf.id;


--
-- Name: sharedrepo; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.sharedrepo (
    id bigint NOT NULL,
    repo_id character(37),
    from_email character varying(255),
    to_email character varying(255),
    permission character(15)
);


ALTER TABLE public.sharedrepo OWNER TO seafile_os_framework;

--
-- Name: sharedrepo_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.sharedrepo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.sharedrepo_id_seq OWNER TO seafile_os_framework;

--
-- Name: sharedrepo_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.sharedrepo_id_seq OWNED BY public.sharedrepo.id;


--
-- Name: systeminfo; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.systeminfo (
    id bigint NOT NULL,
    info_key character varying(256),
    info_value character varying(1024)
);


ALTER TABLE public.systeminfo OWNER TO seafile_os_framework;

--
-- Name: systeminfo_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.systeminfo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.systeminfo_id_seq OWNER TO seafile_os_framework;

--
-- Name: systeminfo_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.systeminfo_id_seq OWNED BY public.systeminfo.id;


--
-- Name: userquota; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.userquota (
    id bigint NOT NULL,
    "user" character varying(255),
    quota bigint
);


ALTER TABLE public.userquota OWNER TO seafile_os_framework;

--
-- Name: userquota_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.userquota_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.userquota_id_seq OWNER TO seafile_os_framework;

--
-- Name: userquota_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.userquota_id_seq OWNED BY public.userquota.id;


--
-- Name: usersharequota; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.usersharequota (
    id bigint NOT NULL,
    "user" character varying(255),
    quota bigint
);


ALTER TABLE public.usersharequota OWNER TO seafile_os_framework;

--
-- Name: usersharequota_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.usersharequota_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.usersharequota_id_seq OWNER TO seafile_os_framework;

--
-- Name: usersharequota_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.usersharequota_id_seq OWNED BY public.usersharequota.id;


--
-- Name: virtualrepo; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.virtualrepo (
    id bigint NOT NULL,
    repo_id character(36),
    origin_repo character(36),
    path text,
    base_commit character(40)
);


ALTER TABLE public.virtualrepo OWNER TO seafile_os_framework;

--
-- Name: virtualrepo_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.virtualrepo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.virtualrepo_id_seq OWNER TO seafile_os_framework;

--
-- Name: virtualrepo_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.virtualrepo_id_seq OWNED BY public.virtualrepo.id;


--
-- Name: webap; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.webap (
    id bigint NOT NULL,
    repo_id character(37),
    access_property character(10)
);


ALTER TABLE public.webap OWNER TO seafile_os_framework;

--
-- Name: webap_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.webap_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.webap_id_seq OWNER TO seafile_os_framework;

--
-- Name: webap_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.webap_id_seq OWNED BY public.webap.id;


--
-- Name: webuploadtempfiles; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.webuploadtempfiles (
    id bigint NOT NULL,
    repo_id character(40) NOT NULL,
    file_path text NOT NULL,
    tmp_file_path text NOT NULL
);


ALTER TABLE public.webuploadtempfiles OWNER TO seafile_os_framework;

--
-- Name: webuploadtempfiles_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.webuploadtempfiles_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.webuploadtempfiles_id_seq OWNER TO seafile_os_framework;

--
-- Name: webuploadtempfiles_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.webuploadtempfiles_id_seq OWNED BY public.webuploadtempfiles.id;


--
-- Name: branch id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.branch ALTER COLUMN id SET DEFAULT nextval('public.branch_id_seq'::regclass);


--
-- Name: garbagerepos id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.garbagerepos ALTER COLUMN id SET DEFAULT nextval('public.garbagerepos_id_seq'::regclass);


--
-- Name: innerpubrepo id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.innerpubrepo ALTER COLUMN id SET DEFAULT nextval('public.innerpubrepo_id_seq'::regclass);


--
-- Name: orgquota id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orgquota ALTER COLUMN id SET DEFAULT nextval('public.orgquota_id_seq'::regclass);


--
-- Name: orguserquota id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orguserquota ALTER COLUMN id SET DEFAULT nextval('public.orguserquota_id_seq'::regclass);


--
-- Name: repo id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repo ALTER COLUMN id SET DEFAULT nextval('public.repo_id_seq'::regclass);


--
-- Name: repofilecount id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repofilecount ALTER COLUMN id SET DEFAULT nextval('public.repofilecount_id_seq'::regclass);


--
-- Name: repogroup id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repogroup ALTER COLUMN id SET DEFAULT nextval('public.repogroup_id_seq'::regclass);


--
-- Name: repohead id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repohead ALTER COLUMN id SET DEFAULT nextval('public.repohead_id_seq'::regclass);


--
-- Name: repohistorylimit id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repohistorylimit ALTER COLUMN id SET DEFAULT nextval('public.repohistorylimit_id_seq'::regclass);


--
-- Name: repoinfo id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repoinfo ALTER COLUMN id SET DEFAULT nextval('public.repoinfo_id_seq'::regclass);


--
-- Name: repoowner id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repoowner ALTER COLUMN id SET DEFAULT nextval('public.repoowner_id_seq'::regclass);


--
-- Name: reposize id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.reposize ALTER COLUMN id SET DEFAULT nextval('public.reposize_id_seq'::regclass);


--
-- Name: repotokenpeerinfo id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repotokenpeerinfo ALTER COLUMN id SET DEFAULT nextval('public.repotokenpeerinfo_id_seq'::regclass);


--
-- Name: repotrash id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repotrash ALTER COLUMN id SET DEFAULT nextval('public.repotrash_id_seq'::regclass);


--
-- Name: repousertoken id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repousertoken ALTER COLUMN id SET DEFAULT nextval('public.repousertoken_id_seq'::regclass);


--
-- Name: repovalidsince id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repovalidsince ALTER COLUMN id SET DEFAULT nextval('public.repovalidsince_id_seq'::regclass);


--
-- Name: seafileconf id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.seafileconf ALTER COLUMN id SET DEFAULT nextval('public.seafileconf_id_seq'::regclass);


--
-- Name: sharedrepo id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sharedrepo ALTER COLUMN id SET DEFAULT nextval('public.sharedrepo_id_seq'::regclass);


--
-- Name: systeminfo id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.systeminfo ALTER COLUMN id SET DEFAULT nextval('public.systeminfo_id_seq'::regclass);


--
-- Name: userquota id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.userquota ALTER COLUMN id SET DEFAULT nextval('public.userquota_id_seq'::regclass);


--
-- Name: usersharequota id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.usersharequota ALTER COLUMN id SET DEFAULT nextval('public.usersharequota_id_seq'::regclass);


--
-- Name: virtualrepo id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.virtualrepo ALTER COLUMN id SET DEFAULT nextval('public.virtualrepo_id_seq'::regclass);


--
-- Name: webap id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.webap ALTER COLUMN id SET DEFAULT nextval('public.webap_id_seq'::regclass);


--
-- Name: webuploadtempfiles id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.webuploadtempfiles ALTER COLUMN id SET DEFAULT nextval('public.webuploadtempfiles_id_seq'::regclass);


--
-- Data for Name: branch; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.branch (id, name, repo_id, commit_id) FROM stdin;
-- 1	master	e242e5d5-72cd-4408-80d3-60861c072da1     	e6573433d7e0279c9f937fb92100f2fcdcea6a1b
-- 2	master	1f5f9668-0958-43a5-ab5d-edd1c2dd6fbd     	3a31767e35c7a840ba5277ce1a68b8d951cdbeed
-- \.


--
-- Data for Name: garbagerepos; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.garbagerepos (id, repo_id) FROM stdin;
-- \.


--
-- Data for Name: innerpubrepo; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.innerpubrepo (id, repo_id, permission) FROM stdin;
-- \.


--
-- Data for Name: orgquota; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.orgquota (id, org_id, quota) FROM stdin;
-- \.


--
-- Data for Name: orguserquota; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.orguserquota (id, org_id, "user", quota) FROM stdin;
-- \.


--
-- Data for Name: repo; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repo (id, repo_id) FROM stdin;
-- 1	e242e5d5-72cd-4408-80d3-60861c072da1
-- 2	1f5f9668-0958-43a5-ab5d-edd1c2dd6fbd
-- \.


--
-- Data for Name: repofilecount; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repofilecount (id, repo_id, file_count) FROM stdin;
-- \.


--
-- Data for Name: repogroup; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repogroup (id, repo_id, group_id, user_name, permission) FROM stdin;
-- \.


--
-- Data for Name: repohead; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repohead (id, repo_id, branch_name) FROM stdin;
-- 1	e242e5d5-72cd-4408-80d3-60861c072da1 	master
-- 2	1f5f9668-0958-43a5-ab5d-edd1c2dd6fbd 	master
-- \.


--
-- Data for Name: repohistorylimit; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repohistorylimit (id, repo_id, days) FROM stdin;
-- \.


--
-- Data for Name: repoinfo; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repoinfo (id, repo_id, name, update_time, version, is_encrypted, last_modifier, status) FROM stdin;
-- 1	e242e5d5-72cd-4408-80d3-60861c072da1	My Library Template	1695280708	1	0	system	0
-- 2	1f5f9668-0958-43a5-ab5d-edd1c2dd6fbd	My Library	1695280756	1	0	d3eab9d29ae248259d0283dc5a43793e@auth.local	0
-- \.


--
-- Data for Name: repoowner; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repoowner (id, repo_id, owner_id) FROM stdin;
-- 1	e242e5d5-72cd-4408-80d3-60861c072da1 	System
-- 2	1f5f9668-0958-43a5-ab5d-edd1c2dd6fbd 	d3eab9d29ae248259d0283dc5a43793e@auth.local
-- \.


--
-- Data for Name: reposize; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.reposize (id, repo_id, size, head_id) FROM stdin;
-- \.


--
-- Data for Name: repotokenpeerinfo; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repotokenpeerinfo (id, token, peer_id, peer_ip, peer_name, sync_time, client_ver) FROM stdin;
-- \.


--
-- Data for Name: repotrash; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repotrash (id, repo_id, repo_name, head_id, owner_id, size, org_id, del_time) FROM stdin;
-- \.


--
-- Data for Name: repousertoken; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repousertoken (id, repo_id, email, token) FROM stdin;
-- \.


--
-- Data for Name: repovalidsince; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.repovalidsince (id, repo_id, "timestamp") FROM stdin;
-- \.


--
-- Data for Name: seafileconf; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.seafileconf (id, cfg_group, cfg_key, value, property) FROM stdin;
-- \.


--
-- Data for Name: sharedrepo; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.sharedrepo (id, repo_id, from_email, to_email, permission) FROM stdin;
-- \.


--
-- Data for Name: systeminfo; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.systeminfo (id, info_key, info_value) FROM stdin;
-- 1	default_repo_id	e242e5d5-72cd-4408-80d3-60861c072da1
-- \.


--
-- Data for Name: userquota; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.userquota (id, "user", quota) FROM stdin;
-- \.


--
-- Data for Name: usersharequota; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.usersharequota (id, "user", quota) FROM stdin;
-- \.


--
-- Data for Name: virtualrepo; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.virtualrepo (id, repo_id, origin_repo, path, base_commit) FROM stdin;
-- \.


--
-- Data for Name: webap; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.webap (id, repo_id, access_property) FROM stdin;
-- \.


--
-- Data for Name: webuploadtempfiles; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.webuploadtempfiles (id, repo_id, file_path, tmp_file_path) FROM stdin;
-- \.


--
-- Name: branch_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.branch_id_seq', 2, true);


--
-- Name: garbagerepos_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.garbagerepos_id_seq', 1, false);


--
-- Name: innerpubrepo_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.innerpubrepo_id_seq', 1, false);


--
-- Name: orgquota_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.orgquota_id_seq', 1, false);


--
-- Name: orguserquota_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.orguserquota_id_seq', 1, false);


--
-- Name: repo_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repo_id_seq', 2, true);


--
-- Name: repofilecount_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repofilecount_id_seq', 1, false);


--
-- Name: repogroup_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repogroup_id_seq', 1, false);


--
-- Name: repohead_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repohead_id_seq', 2, true);


--
-- Name: repohistorylimit_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repohistorylimit_id_seq', 1, false);


--
-- Name: repoinfo_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repoinfo_id_seq', 2, true);


--
-- Name: repoowner_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repoowner_id_seq', 2, true);


--
-- Name: reposize_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.reposize_id_seq', 1, false);


--
-- Name: repotokenpeerinfo_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repotokenpeerinfo_id_seq', 1, false);


--
-- Name: repotrash_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repotrash_id_seq', 1, false);


--
-- Name: repousertoken_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repousertoken_id_seq', 1, false);


--
-- Name: repovalidsince_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.repovalidsince_id_seq', 1, false);


--
-- Name: seafileconf_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.seafileconf_id_seq', 1, false);


--
-- Name: sharedrepo_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.sharedrepo_id_seq', 1, false);


--
-- Name: systeminfo_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.systeminfo_id_seq', 1, true);


--
-- Name: userquota_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.userquota_id_seq', 1, false);


--
-- Name: usersharequota_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.usersharequota_id_seq', 1, false);


--
-- Name: virtualrepo_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.virtualrepo_id_seq', 1, false);


--
-- Name: webap_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.webap_id_seq', 1, false);


--
-- Name: webuploadtempfiles_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.webuploadtempfiles_id_seq', 1, false);


--
-- Name: branch branch_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.branch
    ADD CONSTRAINT branch_pkey PRIMARY KEY (id);


--
-- Name: garbagerepos garbagerepos_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.garbagerepos
    ADD CONSTRAINT garbagerepos_pkey PRIMARY KEY (id);


--
-- Name: innerpubrepo innerpubrepo_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.innerpubrepo
    ADD CONSTRAINT innerpubrepo_pkey PRIMARY KEY (id);


--
-- Name: orgquota orgquota_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orgquota
    ADD CONSTRAINT orgquota_pkey PRIMARY KEY (id);


--
-- Name: orguserquota orguserquota_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orguserquota
    ADD CONSTRAINT orguserquota_pkey PRIMARY KEY (id);


--
-- Name: repo repo_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repo
    ADD CONSTRAINT repo_pkey PRIMARY KEY (id);


--
-- Name: repofilecount repofilecount_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repofilecount
    ADD CONSTRAINT repofilecount_pkey PRIMARY KEY (id);


--
-- Name: repogroup repogroup_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repogroup
    ADD CONSTRAINT repogroup_pkey PRIMARY KEY (id);


--
-- Name: repohead repohead_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repohead
    ADD CONSTRAINT repohead_pkey PRIMARY KEY (id);


--
-- Name: repohistorylimit repohistorylimit_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repohistorylimit
    ADD CONSTRAINT repohistorylimit_pkey PRIMARY KEY (id);


--
-- Name: repoinfo repoinfo_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repoinfo
    ADD CONSTRAINT repoinfo_pkey PRIMARY KEY (id);


--
-- Name: repoowner repoowner_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repoowner
    ADD CONSTRAINT repoowner_pkey PRIMARY KEY (id);


--
-- Name: reposize reposize_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.reposize
    ADD CONSTRAINT reposize_pkey PRIMARY KEY (id);


--
-- Name: repotokenpeerinfo repotokenpeerinfo_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repotokenpeerinfo
    ADD CONSTRAINT repotokenpeerinfo_pkey PRIMARY KEY (id);


--
-- Name: repotrash repotrash_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repotrash
    ADD CONSTRAINT repotrash_pkey PRIMARY KEY (id);


--
-- Name: repousertoken repousertoken_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repousertoken
    ADD CONSTRAINT repousertoken_pkey PRIMARY KEY (id);


--
-- Name: repovalidsince repovalidsince_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.repovalidsince
    ADD CONSTRAINT repovalidsince_pkey PRIMARY KEY (id);


--
-- Name: seafileconf seafileconf_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.seafileconf
    ADD CONSTRAINT seafileconf_pkey PRIMARY KEY (id);


--
-- Name: sharedrepo sharedrepo_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sharedrepo
    ADD CONSTRAINT sharedrepo_pkey PRIMARY KEY (id);


--
-- Name: systeminfo systeminfo_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.systeminfo
    ADD CONSTRAINT systeminfo_pkey PRIMARY KEY (id);


--
-- Name: userquota userquota_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.userquota
    ADD CONSTRAINT userquota_pkey PRIMARY KEY (id);


--
-- Name: usersharequota usersharequota_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.usersharequota
    ADD CONSTRAINT usersharequota_pkey PRIMARY KEY (id);


--
-- Name: virtualrepo virtualrepo_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.virtualrepo
    ADD CONSTRAINT virtualrepo_pkey PRIMARY KEY (id);


--
-- Name: webap webap_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.webap
    ADD CONSTRAINT webap_pkey PRIMARY KEY (id);


--
-- Name: webuploadtempfiles webuploadtempfiles_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.webuploadtempfiles
    ADD CONSTRAINT webuploadtempfiles_pkey PRIMARY KEY (id);

--
-- Name: webuploadtempfiles_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX webuploadtempfiles_repoid_idx ON public.webuploadtempfiles USING btree (repo_id);


--
-- Name: branch_repoidname_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX branch_repoidname_idx ON public.branch USING btree (repo_id, name);


--
-- Name: garbagerepos_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX garbagerepos_repoid_idx ON public.garbagerepos USING btree (repo_id);


--
-- Name: innerpubrepo_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX innerpubrepo_repoid_idx ON public.innerpubrepo USING btree (repo_id);


--
-- Name: orgquota_orgid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX orgquota_orgid_idx ON public.orgquota USING btree (org_id);


--
-- Name: orguserquota_orgid_user_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX orguserquota_orgid_user_idx ON public.orguserquota USING btree (org_id, "user");


--
-- Name: repo_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repo_repoid_idx ON public.repo USING btree (repo_id);


--
-- Name: repofilecount_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repofilecount_repoid_idx ON public.repofilecount USING btree (repo_id);


--
-- Name: repogroup_group_id_repo_id_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repogroup_group_id_repo_id_idx ON public.repogroup USING btree (group_id, repo_id);


--
-- Name: repogroup_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX repogroup_repoid_idx ON public.repogroup USING btree (repo_id);


--
-- Name: repogroup_username_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX repogroup_username_idx ON public.repogroup USING btree (user_name);


--
-- Name: repohead_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repohead_repoid_idx ON public.repohead USING btree (repo_id);


--
-- Name: repohistorylimit_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repohistorylimit_repoid_idx ON public.repohistorylimit USING btree (repo_id);


--
-- Name: repoinfo_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repoinfo_repoid_idx ON public.repoinfo USING btree (repo_id);


--
-- Name: repoowner_owner_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX repoowner_owner_idx ON public.repoowner USING btree (owner_id);


--
-- Name: repoowner_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repoowner_repoid_idx ON public.repoowner USING btree (repo_id);


--
-- Name: reposize_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX reposize_repoid_idx ON public.reposize USING btree (repo_id);


--
-- Name: repotokenpeerinfo_peerid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX repotokenpeerinfo_peerid_idx ON public.repotokenpeerinfo USING btree (peer_id);


--
-- Name: repotokenpeerinfo_token_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repotokenpeerinfo_token_idx ON public.repotokenpeerinfo USING btree (token);


--
-- Name: repotrash_org_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX repotrash_org_id ON public.repotrash USING btree (org_id);


--
-- Name: repotrash_owner_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX repotrash_owner_id ON public.repotrash USING btree (owner_id);


--
-- Name: repotrash_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repotrash_repoid_idx ON public.repotrash USING btree (repo_id);


--
-- Name: repousertoken_email_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX repousertoken_email_idx ON public.repousertoken USING btree (email);


--
-- Name: repousertoken_repo_id_token_key; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repousertoken_repo_id_token_key ON public.repousertoken USING btree (repo_id, token);


--
-- Name: repousertoken_token_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX repousertoken_token_idx ON public.repousertoken USING btree (token);


--
-- Name: repovalidsince_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX repovalidsince_repoid_idx ON public.repovalidsince USING btree (repo_id);


--
-- Name: sharedrepo_from_email_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX sharedrepo_from_email_idx ON public.sharedrepo USING btree (from_email);


--
-- Name: sharedrepo_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX sharedrepo_repoid_idx ON public.sharedrepo USING btree (repo_id);


--
-- Name: sharedrepo_to_email_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX sharedrepo_to_email_idx ON public.sharedrepo USING btree (to_email);


--
-- Name: userquota_user_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX userquota_user_idx ON public.userquota USING btree ("user");


--
-- Name: usersharequota_user_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX usersharequota_user_idx ON public.usersharequota USING btree ("user");


--
-- Name: virtualrepo_origin_repo_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX virtualrepo_origin_repo_idx ON public.virtualrepo USING btree (origin_repo);


--
-- Name: virtualrepo_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX virtualrepo_repoid_idx ON public.virtualrepo USING btree (repo_id);


--
-- Name: webap_repoid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX webap_repoid_idx ON public.webap USING btree (repo_id);


--
-- PostgreSQL database dump complete
--
