--
-- PostgreSQL database dump
--

-- Dumped from database version 14.9 (Ubuntu 14.9-0ubuntu0.22.04.1)
-- Dumped by pg_dump version 14.9 (Ubuntu 14.9-0ubuntu0.22.04.1)

-- SET statement_timeout = 0;
-- SET lock_timeout = 0;
-- SET idle_in_transaction_session_timeout = 0;
-- SET client_encoding = 'UTF8';
-- SET standard_conforming_strings = on;
-- SELECT pg_catalog.set_config('search_path', '', false);
-- SET check_function_bodies = false;
-- SET xmloption = content;
-- SET client_min_messages = warning;
-- SET row_security = off;

--
-- Name: ccnet; Type: DATABASE; Schema: -; Owner: postgres
--

-- CREATE DATABASE ccnet WITH TEMPLATE = template0 ENCODING = 'UTF8' LOCALE = 'C.UTF-8';


-- ALTER DATABASE ccnet OWNER TO postgres;

\connect os_framework_ccnet

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
-- Name: Group; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public."Group" (
    group_id bigint NOT NULL,
    group_name character varying(255),
    creator_name character varying(255),
    "timestamp" bigint,
    type character varying(32),
    parent_group_id integer
);


ALTER TABLE public."Group" OWNER TO seafile_os_framework;

--
-- Name: Group_group_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public."Group_group_id_seq"
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public."Group_group_id_seq" OWNER TO seafile_os_framework;

--
-- Name: Group_group_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public."Group_group_id_seq" OWNED BY public."Group".group_id;


--
-- Name: binding; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.binding (
    id bigint NOT NULL,
    email character varying(255),
    peer_id character(41)
);


ALTER TABLE public.binding OWNER TO seafile_os_framework;

--
-- Name: binding_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.binding_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.binding_id_seq OWNER TO seafile_os_framework;

--
-- Name: binding_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.binding_id_seq OWNED BY public.binding.id;


--
-- Name: emailuser; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.emailuser (
    id bigint NOT NULL,
    email character varying(255),
    passwd character varying(256),
    is_staff smallint NOT NULL,
    is_active smallint NOT NULL,
    is_department_owner smallint NOT NULL DEFAULT 0,
    ctime bigint,
    reference_id character varying(255)
);

ALTER TABLE public.emailuser OWNER TO seafile_os_framework;

--
-- Name: emailuser_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.emailuser_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.emailuser_id_seq OWNER TO seafile_os_framework;

--
-- Name: emailuser_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.emailuser_id_seq OWNED BY public.emailuser.id;

--
-- Name: emailuser_is_active_idx; Type: INDEX; Schema: public; Owner: seafile_os_framework
--

CREATE INDEX emailuser_is_active_idx ON public.EmailUser USING btree (is_active);

--
-- Name: emailuser_is_department_owner_idx; Type: INDEX; Schema: public; Owner: seafile_os_framework
--

CREATE INDEX emailuser_is_department_owner_idx ON public.EmailUser USING btree (is_department_owner);


--
-- Name: groupdnpair; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.groupdnpair (
    id bigint NOT NULL,
    group_id integer,
    dn character varying(255)
);


ALTER TABLE public.groupdnpair OWNER TO seafile_os_framework;

--
-- Name: groupdnpair_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.groupdnpair_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.groupdnpair_id_seq OWNER TO seafile_os_framework;

--
-- Name: groupdnpair_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.groupdnpair_id_seq OWNED BY public.groupdnpair.id;


--
-- Name: groupstructure; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.groupstructure (
    id bigint NOT NULL,
    group_id integer,
    path character varying(1024)
);


ALTER TABLE public.groupstructure OWNER TO seafile_os_framework;

--
-- Name: groupstructure_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.groupstructure_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.groupstructure_id_seq OWNER TO seafile_os_framework;

--
-- Name: groupstructure_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.groupstructure_id_seq OWNED BY public.groupstructure.id;


--
-- Name: groupuser; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.groupuser (
    id bigint NOT NULL,
    group_id bigint,
    user_name character varying(255),
    is_staff smallint
);


ALTER TABLE public.groupuser OWNER TO seafile_os_framework;

--
-- Name: groupuser_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.groupuser_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.groupuser_id_seq OWNER TO seafile_os_framework;

--
-- Name: groupuser_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.groupuser_id_seq OWNED BY public.groupuser.id;


--
-- Name: ldapconfig; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.ldapconfig (
    id bigint NOT NULL,
    cfg_group character varying(255) NOT NULL,
    cfg_key character varying(255) NOT NULL,
    value character varying(255),
    property integer
);


ALTER TABLE public.ldapconfig OWNER TO seafile_os_framework;

--
-- Name: ldapconfig_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.ldapconfig_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.ldapconfig_id_seq OWNER TO seafile_os_framework;

--
-- Name: ldapconfig_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.ldapconfig_id_seq OWNED BY public.ldapconfig.id;


--
-- Name: ldapusers; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.ldapusers (
    id bigint NOT NULL,
    email character varying(255) NOT NULL,
    password character varying(255) NOT NULL,
    is_staff smallint NOT NULL,
    is_active smallint NOT NULL,
    extra_attrs text,
    reference_id character varying(255)
);


ALTER TABLE public.ldapusers OWNER TO seafile_os_framework;

--
-- Name: ldapusers_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.ldapusers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.ldapusers_id_seq OWNER TO seafile_os_framework;

--
-- Name: ldapusers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.ldapusers_id_seq OWNED BY public.ldapusers.id;


--
-- Name: organization; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.organization (
    org_id bigint NOT NULL,
    org_name character varying(255),
    url_prefix character varying(255),
    creator character varying(255),
    ctime bigint
);


ALTER TABLE public.organization OWNER TO seafile_os_framework;

--
-- Name: organization_org_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.organization_org_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.organization_org_id_seq OWNER TO seafile_os_framework;

--
-- Name: organization_org_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.organization_org_id_seq OWNED BY public.organization.org_id;


--
-- Name: orggroup; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.orggroup (
    id bigint NOT NULL,
    org_id integer,
    group_id integer
);


ALTER TABLE public.orggroup OWNER TO seafile_os_framework;

--
-- Name: orggroup_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.orggroup_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.orggroup_id_seq OWNER TO seafile_os_framework;

--
-- Name: orggroup_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.orggroup_id_seq OWNED BY public.orggroup.id;


--
-- Name: orguser; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.orguser (
    id bigint NOT NULL,
    org_id integer,
    email character varying(255),
    is_staff smallint NOT NULL
);


ALTER TABLE public.orguser OWNER TO seafile_os_framework;

--
-- Name: orguser_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.orguser_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.orguser_id_seq OWNER TO seafile_os_framework;

--
-- Name: orguser_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.orguser_id_seq OWNED BY public.orguser.id;


--
-- Name: userrole; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.userrole (
    id bigint NOT NULL,
    email character varying(255),
    role character varying(255),
    is_manual_set smallint DEFAULT 0
);


ALTER TABLE public.userrole OWNER TO seafile_os_framework;

--
-- Name: userrole_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.userrole_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.userrole_id_seq OWNER TO seafile_os_framework;

--
-- Name: userrole_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.userrole_id_seq OWNED BY public.userrole.id;


--
-- Name: Group group_id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public."Group" ALTER COLUMN group_id SET DEFAULT nextval('public."Group_group_id_seq"'::regclass);


--
-- Name: binding id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.binding ALTER COLUMN id SET DEFAULT nextval('public.binding_id_seq'::regclass);


--
-- Name: emailuser id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.emailuser ALTER COLUMN id SET DEFAULT nextval('public.emailuser_id_seq'::regclass);


--
-- Name: groupdnpair id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.groupdnpair ALTER COLUMN id SET DEFAULT nextval('public.groupdnpair_id_seq'::regclass);


--
-- Name: groupstructure id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.groupstructure ALTER COLUMN id SET DEFAULT nextval('public.groupstructure_id_seq'::regclass);


--
-- Name: groupuser id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.groupuser ALTER COLUMN id SET DEFAULT nextval('public.groupuser_id_seq'::regclass);


--
-- Name: ldapconfig id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.ldapconfig ALTER COLUMN id SET DEFAULT nextval('public.ldapconfig_id_seq'::regclass);


--
-- Name: ldapusers id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.ldapusers ALTER COLUMN id SET DEFAULT nextval('public.ldapusers_id_seq'::regclass);


--
-- Name: organization org_id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization ALTER COLUMN org_id SET DEFAULT nextval('public.organization_org_id_seq'::regclass);


--
-- Name: orggroup id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orggroup ALTER COLUMN id SET DEFAULT nextval('public.orggroup_id_seq'::regclass);


--
-- Name: orguser id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orguser ALTER COLUMN id SET DEFAULT nextval('public.orguser_id_seq'::regclass);


--
-- Name: userrole id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.userrole ALTER COLUMN id SET DEFAULT nextval('public.userrole_id_seq'::regclass);


--
-- Data for Name: Group; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public."Group" (group_id, group_name, creator_name, "timestamp", type, parent_group_id) FROM stdin;
-- \.


--
-- Data for Name: binding; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.binding (id, email, peer_id) FROM stdin;
-- \.


--
-- Data for Name: emailuser; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.emailuser (id, email, passwd, is_staff, is_active, ctime, reference_id) FROM stdin;
-- 1	d3eab9d29ae248259d0283dc5a43793e@auth.local	PBKDF2SHA256$10000$b45b945eb8f5c264ca1e0bf15077379f49e55f907a040a3ef13dc58acddf73c5$0754261f77f97dec861a2fd9d3175bf14f3c96ba8c8da070bd7473b35e27b97c	1	1	1695280735595098	\N
-- \.


--
-- Data for Name: groupdnpair; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.groupdnpair (id, group_id, dn) FROM stdin;
-- \.


--
-- Data for Name: groupstructure; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.groupstructure (id, group_id, path) FROM stdin;
-- \.


--
-- Data for Name: groupuser; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.groupuser (id, group_id, user_name, is_staff) FROM stdin;
-- \.


--
-- Data for Name: ldapconfig; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.ldapconfig (id, cfg_group, cfg_key, value, property) FROM stdin;
-- \.


--
-- Data for Name: ldapusers; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.ldapusers (id, email, password, is_staff, is_active, extra_attrs, reference_id) FROM stdin;
-- \.


--
-- Data for Name: organization; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.organization (org_id, org_name, url_prefix, creator, ctime) FROM stdin;
-- \.


--
-- Data for Name: orggroup; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.orggroup (id, org_id, group_id) FROM stdin;
-- \.


--
-- Data for Name: orguser; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.orguser (id, org_id, email, is_staff) FROM stdin;
-- \.


--
-- Data for Name: userrole; Type: TABLE DATA; Schema: public; Owner: postgres
--

-- COPY public.userrole (id, email, role, is_manual_set) FROM stdin;
-- \.


--
-- Name: Group_group_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public."Group_group_id_seq"', 1, false);


--
-- Name: binding_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.binding_id_seq', 1, false);


--
-- Name: emailuser_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.emailuser_id_seq', 1, true);


--
-- Name: groupdnpair_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.groupdnpair_id_seq', 1, false);


--
-- Name: groupstructure_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.groupstructure_id_seq', 1, false);


--
-- Name: groupuser_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.groupuser_id_seq', 1, false);


--
-- Name: ldapconfig_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.ldapconfig_id_seq', 1, false);


--
-- Name: ldapusers_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.ldapusers_id_seq', 1, false);


--
-- Name: organization_org_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.organization_org_id_seq', 1, false);


--
-- Name: orggroup_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.orggroup_id_seq', 1, false);


--
-- Name: orguser_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.orguser_id_seq', 1, false);


--
-- Name: userrole_id_seq; Type: SEQUENCE SET; Schema: public; Owner: postgres
--

-- SELECT pg_catalog.setval('public.userrole_id_seq', 1, false);


--
-- Name: Group Group_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public."Group"
    ADD CONSTRAINT "Group_pkey" PRIMARY KEY (group_id);


--
-- Name: binding binding_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.binding
    ADD CONSTRAINT binding_pkey PRIMARY KEY (id);


--
-- Name: emailuser emailuser_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.emailuser
    ADD CONSTRAINT emailuser_pkey PRIMARY KEY (id);


--
-- Name: groupdnpair groupdnpair_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.groupdnpair
    ADD CONSTRAINT groupdnpair_pkey PRIMARY KEY (id);


--
-- Name: groupstructure groupstructure_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.groupstructure
    ADD CONSTRAINT groupstructure_pkey PRIMARY KEY (id);


--
-- Name: groupuser groupuser_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.groupuser
    ADD CONSTRAINT groupuser_pkey PRIMARY KEY (id);


--
-- Name: ldapconfig ldapconfig_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.ldapconfig
    ADD CONSTRAINT ldapconfig_pkey PRIMARY KEY (id);


--
-- Name: ldapusers ldapusers_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.ldapusers
    ADD CONSTRAINT ldapusers_pkey PRIMARY KEY (id);


--
-- Name: organization organization_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organization
    ADD CONSTRAINT organization_pkey PRIMARY KEY (org_id);


--
-- Name: orggroup orggroup_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orggroup
    ADD CONSTRAINT orggroup_pkey PRIMARY KEY (id);


--
-- Name: orguser orguser_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.orguser
    ADD CONSTRAINT orguser_pkey PRIMARY KEY (id);


--
-- Name: userrole userrole_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.userrole
    ADD CONSTRAINT userrole_pkey PRIMARY KEY (id);


--
-- Name: binding_email_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX binding_email_idx ON public.binding USING btree (email);


--
-- Name: binding_peerid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX binding_peerid_idx ON public.binding USING btree (peer_id);


--
-- Name: emailuser_email_key; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX emailuser_email_key ON public.emailuser USING btree (email);


--
-- Name: emailuser_reference_id_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX emailuser_reference_id_idx ON public.emailuser USING btree (reference_id);


--
-- Name: groupstructure_groupid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX groupstructure_groupid_idx ON public.groupstructure USING btree (group_id);


--
-- Name: groupuser_group_id_user_name_key; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX groupuser_group_id_user_name_key ON public.groupuser USING btree (group_id, user_name);


--
-- Name: groupuser_username_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX groupuser_username_idx ON public.groupuser USING btree (user_name);


--
-- Name: ldapusers_email_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ldapusers_email_idx ON public.ldapusers USING btree (email);


--
-- Name: ldapusers_reference_id_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ldapusers_reference_id_idx ON public.ldapusers USING btree (reference_id);


--
-- Name: organization_url_prefix_key; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX organization_url_prefix_key ON public.organization USING btree (url_prefix);


--
-- Name: orggroup_groupid_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX orggroup_groupid_idx ON public.orggroup USING btree (group_id);


--
-- Name: orggroup_org_id_group_id_key; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX orggroup_org_id_group_id_key ON public.orggroup USING btree (org_id, group_id);


--
-- Name: orguser_email_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX orguser_email_idx ON public.orguser USING btree (email);


--
-- Name: orguser_org_id_email_key; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX orguser_org_id_email_key ON public.orguser USING btree (org_id, email);


--
-- Name: structure_path_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX structure_path_idx ON public.groupstructure USING btree (path);


--
-- Name: userrole_email_idx; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX userrole_email_idx ON public.userrole USING btree (email);


--
-- Name: userrole_email_role_key; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX userrole_email_role_key ON public.userrole USING btree (email, role);


--
-- PostgreSQL database dump complete
--
