cmake_minimum_required(VERSION 3.22)
project(CardStoreTests LANGUAGES CXX)
set(CMAKE_CXX_STANDARD 17)
set(CMAKE_AUTOMOC ON)
find_package(Qt6 REQUIRED COMPONENTS Core Sql Qml Quick QuickControls2 Network Test)
find_package(KF6Kirigami CONFIG REQUIRED)
enable_testing()
set(LB_TEST_PLATFORM "offscreen" CACHE STRING "Qt platform used by CTest")
add_executable(tst_cardstore tst_cardstore.cpp ../src/CardStore.cpp ../src/CardStore.h ../src/api.cpp ../src/api.h ../src/cardstoretransport.h)
target_include_directories(tst_cardstore PRIVATE ../src)
target_link_libraries(tst_cardstore PRIVATE Qt6::Core Qt6::Sql Qt6::Qml Qt6::Quick Qt6::QuickControls2 Qt6::Network Qt6::Test KF6::Kirigami)
target_compile_definitions(tst_cardstore PRIVATE LB_SOURCE_ACTIONS_QML="${CMAKE_CURRENT_SOURCE_DIR}/../src/qml/CardActions.qml")
add_test(NAME cardstore COMMAND tst_cardstore)
set_tests_properties(cardstore PROPERTIES ENVIRONMENT "QT_QPA_PLATFORM=${LB_TEST_PLATFORM}")

# Opt-in measurement (not an offscreen CTest): on an owned systemd-run Xvfb,
# python3 mailresize_driver.py --runner <build>/tst_mailresize --display :N --output <proof-dir>
# reports 10-second real-mail resizes and inbox/open/close/archive/create heartbeat gaps.
# --imports <5fadb81>/qt/tests/qml-imports repeats the same oracle as a negative control.
if(NOT ANDROID)
    find_package(Qt6 REQUIRED COMPONENTS QuickTest WebEngineQuick)
    add_executable(tst_mailresize maildetail_runner.cpp ../src/maildocumentprofile.cpp ../src/maildocumentprofile.h
                   ../src/CardStore.cpp ../src/CardStore.h)
    target_include_directories(tst_mailresize PRIVATE ../src)
    target_compile_definitions(tst_mailresize PRIVATE LB_RESIZE_STORE
                              LB_MAIL_FIXTURES_DIR="${CMAKE_CURRENT_SOURCE_DIR}/fixtures")
    target_link_libraries(tst_mailresize PRIVATE Qt6::QuickTest Qt6::WebEngineQuick Qt6::Sql KF6::Kirigami)
    add_test(NAME uipolish COMMAND tst_mailresize
        -input ${CMAKE_CURRENT_SOURCE_DIR}/uipolish.qml
        -import ${CMAKE_CURRENT_SOURCE_DIR}/qml-imports UiPolish::test_geometry)
    set_tests_properties(uipolish PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=${LB_TEST_PLATFORM};QT_QUICK_CONTROLS_STYLE=org.kde.desktop")
    add_test(NAME gmailreconnect COMMAND tst_mailresize
        -input ${CMAKE_CURRENT_SOURCE_DIR}/uipolish.qml
        -import ${CMAKE_CURRENT_SOURCE_DIR}/qml-imports UiPolish::test_reconnectLifecycle)
    set_tests_properties(gmailreconnect PROPERTIES ENVIRONMENT
        "QT_QPA_PLATFORM=${LB_TEST_PLATFORM};QT_QUICK_CONTROLS_STYLE=org.kde.desktop")
endif()

# Real CardStore consumer proof; optionally load a copied desktop cache.
# Consumer contract: bundle fixtures carry state=open; important members stay
# standalone but participate in Archive/Undo, while pinned members never do.
# Pointer consumers wait for adaptive layouts to polish/render, not just for a
# delegate to exist or StackView.depth/busy to settle. Keep the real click,
# original subject/topic, exact anchor, 48px target and popup/palette oracles.
find_package(Qt6 REQUIRED COMPONENTS QuickTest)
add_executable(tst_bundleexpand bundle_runner.cpp ../src/CardStore.cpp ../src/CardStore.h)
target_include_directories(tst_bundleexpand PRIVATE ../src)
target_compile_definitions(tst_bundleexpand PRIVATE
    LB_PHOTO_FIXTURE="${CMAKE_CURRENT_SOURCE_DIR}/fixtures/sender-photo-64x64.png")
target_link_libraries(tst_bundleexpand PRIVATE Qt6::QuickTest Qt6::Sql KF6::Kirigami)
add_test(NAME bundleexpand COMMAND tst_bundleexpand
    -input ${CMAKE_CURRENT_SOURCE_DIR}/tst_bundleexpand.qml
    -import ${CMAKE_CURRENT_SOURCE_DIR}/qml-imports)
set_tests_properties(bundleexpand PROPERTIES ENVIRONMENT
    "QT_QPA_PLATFORM=${LB_TEST_PLATFORM};QT_QUICK_CONTROLS_STYLE=org.kde.desktop")
add_test(NAME markspam COMMAND tst_bundleexpand
    -input ${CMAKE_CURRENT_SOURCE_DIR}/markspam.qml
    -import ${CMAKE_CURRENT_SOURCE_DIR}/qml-imports)
set_tests_properties(markspam PROPERTIES ENVIRONMENT
    "QT_QPA_PLATFORM=${LB_TEST_PLATFORM};QT_QUICK_CONTROLS_STYLE=org.kde.desktop")
# R67 sender-photo consumer regression: first online paint after cache-download
# completion must appear without a manual refresh(); offline cached paint and
# the default/missing/auth-state initials fallbacks must survive in one run.
add_test(NAME senderphotopaint COMMAND tst_bundleexpand
    -input ${CMAKE_CURRENT_SOURCE_DIR}/tst_senderphotopaint.qml
    -import ${CMAKE_CURRENT_SOURCE_DIR}/qml-imports)
set_tests_properties(senderphotopaint PROPERTIES
    TIMEOUT 120
    ENVIRONMENT "QT_QPA_PLATFORM=${LB_TEST_PLATFORM};QT_QUICK_CONTROLS_STYLE=org.kde.desktop")

# Same real CardStore runner, with the large-bundle timing and wheel oracle.
add_test(NAME scrollresilience COMMAND tst_bundleexpand
    -input ${CMAKE_CURRENT_SOURCE_DIR}/tst_scrollresilience.qml
    -import ${CMAKE_CURRENT_SOURCE_DIR}/qml-imports)
set_tests_properties(scrollresilience PROPERTIES
    TIMEOUT 60
    ENVIRONMENT "QT_QPA_PLATFORM=${LB_TEST_PLATFORM};QT_QUICK_CONTROLS_STYLE=org.kde.desktop")
